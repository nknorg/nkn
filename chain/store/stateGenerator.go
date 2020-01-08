package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/chain"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/program"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
)

func (cs *ChainStore) spendTransaction(states *StateDB, txn *transaction.Transaction, totalFee Fixed64, genesis bool, height uint32) error {
	pl, err := transaction.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return err
	}

	switch txn.UnsignedTx.Payload.Type {
	case pb.COINBASE_TYPE:
		coinbase := pl.(*pb.Coinbase)
		if !genesis {
			var donationAmount Fixed64
			donationAmount, err = cs.GetDonation()
			if err != nil {
				return err
			}

			if err = states.UpdateBalance(BytesToUint160(coinbase.Sender), config.NKNAssetID, donationAmount, Subtraction); err != nil {
				return err
			}

		}
		states.IncrNonce(BytesToUint160(coinbase.Sender))
		states.UpdateBalance(BytesToUint160(coinbase.Recipient), config.NKNAssetID, Fixed64(coinbase.Amount)+totalFee, Addition)

		if height != 0 {
			err = states.increaseTotalSupply(config.NKNAssetID, chain.GetRewardByHeight(height))
			if err != nil {
				return err
			}
		}
	case pb.TRANSFER_ASSET_TYPE:
		transfer := pl.(*pb.TransferAsset)
		states.UpdateBalance(BytesToUint160(transfer.Sender), config.NKNAssetID, Fixed64(transfer.Amount)+Fixed64(txn.UnsignedTx.Fee), Subtraction)
		states.IncrNonce(BytesToUint160(transfer.Sender))
		states.UpdateBalance(BytesToUint160(transfer.Recipient), config.NKNAssetID, Fixed64(transfer.Amount), Addition)

	case pb.REGISTER_NAME_TYPE:
		pg, err := txn.GetProgramHashes()
		if err != nil {
			return err
		}

		registerNamePayload := pl.(*pb.RegisterName)
		if err = states.UpdateBalance(pg[0], config.NKNAssetID, Fixed64(registerNamePayload.RegistrationFee)+Fixed64(txn.UnsignedTx.Fee), Subtraction); err != nil {
			return err
		}
		donationAddress, err := ToScriptHash(config.DonationAddress)
		if err != nil {
			return err
		}
		if err = states.UpdateBalance(donationAddress, config.NKNAssetID, Fixed64(registerNamePayload.RegistrationFee), Addition); err != nil {
			return err
		}

		states.IncrNonce(pg[0])

		if config.LegacyNameService.GetValueAtHeight(height) {
			states.setName_legacy(registerNamePayload.Registrant, registerNamePayload.Name)
		} else {
			return states.registerName(registerNamePayload.Name, registerNamePayload.Registrant, config.MaxNameDuration+height)
		}

	case pb.TRANSFER_NAME_TYPE:
		pg, err := txn.GetProgramHashes()
		if err != nil {
			return err
		}

		transferNamePayload := pl.(*pb.TransferName)
		if err := states.UpdateBalance(pg[0], config.NKNAssetID, Fixed64(txn.UnsignedTx.Fee), Subtraction); err != nil {
			return err
		}
		states.IncrNonce(pg[0])

		return states.transferName(transferNamePayload.Name, transferNamePayload.Recipient)

	case pb.DELETE_NAME_TYPE:
		pg, err := txn.GetProgramHashes()
		if err != nil {
			return err
		}

		if err := states.UpdateBalance(pg[0], config.NKNAssetID, Fixed64(txn.UnsignedTx.Fee), Subtraction); err != nil {
			return err
		}
		states.IncrNonce(pg[0])

		deleteNamePayload := pl.(*pb.DeleteName)
		states.deleteNameForRegistrant_legacy(deleteNamePayload.Registrant, deleteNamePayload.Name)
	case pb.SUBSCRIBE_TYPE:
		pg, err := txn.GetProgramHashes()
		if err != nil {
			return err
		}

		if err = states.UpdateBalance(pg[0], config.NKNAssetID, Fixed64(txn.UnsignedTx.Fee), Subtraction); err != nil {
			return err
		}
		states.IncrNonce(pg[0])

		subscribePayload := pl.(*pb.Subscribe)
		err = states.subscribe(subscribePayload.Topic, subscribePayload.Bucket, subscribePayload.Subscriber, subscribePayload.Identifier, subscribePayload.Meta, height+subscribePayload.Duration)
		if err != nil {
			return err
		}
	case pb.UNSUBSCRIBE_TYPE:
		pg, err := txn.GetProgramHashes()
		if err != nil {
			return err
		}

		if err = states.UpdateBalance(pg[0], config.NKNAssetID, Fixed64(txn.UnsignedTx.Fee), Subtraction); err != nil {
			return err
		}
		states.IncrNonce(pg[0])

		unsubscribePayload := pl.(*pb.Unsubscribe)
		err = states.unsubscribe(unsubscribePayload.Topic, unsubscribePayload.Subscriber, unsubscribePayload.Identifier)
		if err != nil {
			return err
		}
	case pb.GENERATE_ID_TYPE:
		genID := pl.(*pb.GenerateID)
		pg, err := txn.GetProgramHashes()
		if err != nil {
			return err
		}

		if err = states.UpdateBalance(pg[0], config.NKNAssetID, Fixed64(genID.RegistrationFee)+Fixed64(txn.UnsignedTx.Fee), Subtraction); err != nil {
			return err
		}
		states.IncrNonce(pg[0])
		states.UpdateID(pg[0], crypto.Sha256ZeroHash)

		donationAddress, err := ToScriptHash(config.DonationAddress)
		if err != nil {
			return err
		}
		states.UpdateBalance(donationAddress, config.NKNAssetID, Fixed64(genID.RegistrationFee), Addition)

	case pb.NANO_PAY_TYPE:
		nanoPay := pl.(*pb.NanoPay)
		pg, err := txn.GetProgramHashes()
		if err != nil {
			return err
		}

		addrRecipient := BytesToUint160(nanoPay.Recipient)
		nanoPayBalance, _, err := states.GetNanoPay(pg[0], addrRecipient, nanoPay.Id)
		if err != nil {
			return err
		}
		claimAmount := Fixed64(nanoPay.Amount) - nanoPayBalance
		if err := states.UpdateBalance(pg[0], config.NKNAssetID, claimAmount, Subtraction); err != nil {
			return err
		}
		if err := states.UpdateBalance(addrRecipient, config.NKNAssetID, claimAmount, Addition); err != nil {
			return err
		}
		if err := states.SetNanoPay(pg[0], addrRecipient, nanoPay.Id, Fixed64(nanoPay.Amount), nanoPay.NanoPayExpiration); err != nil {
			return err
		}
	case pb.ISSUE_ASSET_TYPE:
		issue := pl.(*pb.IssueAsset)
		pg, err := txn.GetProgramHashes()
		if err != nil {
			return err
		}
		h := txn.Hash()
		a, _ := pg[0].ToAddress()
		fmt.Println(h.ToHexString(), a)
		err = states.SetAsset(txn.Hash(), issue.Name, issue.Symbol, Fixed64(issue.TotalSupply), issue.Precision, pg[0])
		if err != nil {
			return err
		}

		err = states.UpdateBalance(pg[0], txn.Hash(), Fixed64(issue.TotalSupply), Addition)
		if err != nil {
			return err
		}

		states.IncrNonce(pg[0])
	}

	return nil
}

func (cs *ChainStore) GenerateStateRoot(ctx context.Context, b *block.Block, genesisBlockInitialized, needBeCommitted bool) (Uint256, error) {
	_, root, err := cs.generateStateRoot(ctx, b, genesisBlockInitialized, needBeCommitted)

	return root, err
}

func (cs *ChainStore) generateStateRoot(ctx context.Context, b *block.Block, genesisBlockInitialized, needBeCommitted bool) (*StateDB, Uint256, error) {
	stateRoot := EmptyUint256
	if genesisBlockInitialized {
		var err error
		stateRoot, err = cs.GetCurrentBlockStateRoot()
		if err != nil {
			return nil, EmptyUint256, err
		}
	}
	states, err := NewStateDB(stateRoot, cs)
	if err != nil {
		return nil, EmptyUint256, err
	}

	//process previous block
	height := b.Header.UnsignedHeader.Height
	if height == 0 {
		var pk *crypto.PubKey
		pk, err = crypto.NewPubKeyFromBytes(b.Header.UnsignedHeader.SignerPk)
		if err != nil {
			return nil, EmptyUint256, err
		}

		var programHash Uint160
		programHash, err = program.CreateProgramHash(pk)
		if err != nil {
			return nil, EmptyUint256, err
		}

		if err = states.UpdateID(programHash, b.Header.UnsignedHeader.SignerId); err != nil {
			return nil, EmptyUint256, err
		}

		var issueAddress Uint160
		issueAddress, err = ToScriptHash(config.InitialIssueAddress)
		if err != nil {
			return nil, EmptyUint256, fmt.Errorf("parse InitialIssueAddress error: %v", err)
		}

		err = states.SetAsset(config.NKNAssetID, config.NKNAssetName, config.NKNAssetSymbol, config.InitialIssueAmount, config.NKNAssetPrecision, issueAddress)
		if err != nil {
			return nil, EmptyUint256, err
		}

		if err = states.UpdateBalance(issueAddress, config.NKNAssetID, config.InitialIssueAmount, Addition); err != nil {
			return nil, EmptyUint256, err
		}

		err = states.SetAsset(config.GASAssetID, config.GASAssetName, config.GASAssetSymbol, 0, config.GASAssetPrecision, issueAddress)
		if err != nil {
			return nil, EmptyUint256, err
		}
	}

	if height > config.GenerateIDBlockDelay {
		var prevBlock *block.Block
		prevBlock, err = cs.GetBlockByHeight(height - config.GenerateIDBlockDelay)
		if err != nil {
			return nil, EmptyUint256, err
		}

		preBlockHash := prevBlock.Hash()

		for _, txn := range prevBlock.Transactions {
			if txn.UnsignedTx.Payload.Type == pb.GENERATE_ID_TYPE {
				select {
				case <-ctx.Done():
					return nil, EmptyUint256, errors.New("context deadline exceeded")
				default:
				}

				id := block.ComputeID(preBlockHash, txn.Hash(), b.Header.UnsignedHeader.RandomBeacon[:config.RandomBeaconUniqueLength])

				var pg []Uint160
				pg, err = txn.GetProgramHashes()
				if err != nil {
					return nil, EmptyUint256, err
				}

				if err = states.UpdateID(pg[0], id); err != nil {
					return nil, EmptyUint256, err
				}
			}
		}
	}

	var totalFee Fixed64
	for _, txn := range b.Transactions {
		totalFee += Fixed64(txn.UnsignedTx.Fee)
	}

	if height == 0 { //genesisBlock
		if err = cs.spendTransaction(states, b.Transactions[0], totalFee, true, height); err != nil {
			return nil, EmptyUint256, err
		}
		for _, txn := range b.Transactions[1:] {
			select {
			case <-ctx.Done():
				return nil, EmptyUint256, errors.New("context deadline exceeded")
			default:
			}

			if err = cs.spendTransaction(states, txn, 0, false, height); err != nil {
				return nil, EmptyUint256, err
			}
		}
	} else {
		for _, txn := range b.Transactions {
			select {
			case <-ctx.Done():
				return nil, EmptyUint256, errors.New("context deadline exceeded")
			default:
			}

			if err = cs.spendTransaction(states, txn, totalFee, false, height); err != nil {
				return nil, EmptyUint256, err
			}
		}

		if err = states.CleanupNanoPay(b.Header.UnsignedHeader.Height); err != nil {
			return nil, EmptyUint256, err
		}

		if err = states.CleanupPubSub(b.Header.UnsignedHeader.Height); err != nil {
			return nil, EmptyUint256, err
		}

		if err = states.CleanupNames(b.Header.UnsignedHeader.Height); err != nil {
			return nil, EmptyUint256, err
		}
	}

	var root Uint256
	if needBeCommitted {
		root, err = states.Finalize(true)
		if err != nil {
			return nil, EmptyUint256, err
		}
	} else {
		root = states.IntermediateRoot()
	}

	return states, root, nil
}
