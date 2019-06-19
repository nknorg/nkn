package db

import (
	"github.com/nknorg/nkn/block"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/vm/contract"
)

func (cs *ChainStore) spendTransaction(states *StateDB, txn *transaction.Transaction, totalFee Fixed64, genesis bool) error {
	pl, err := transaction.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return err
	}

	switch txn.UnsignedTx.Payload.Type {
	case pb.CoinbaseType:
		coinbase := pl.(*pb.Coinbase)
		if !genesis {
			donationAmount, err := cs.GetDonation()
			if err != nil {
				return err
			}
			if err := states.UpdateBalance(BytesToUint160(coinbase.Sender), donationAmount, Subtraction); err != nil {
				return err
			}

		}
		states.IncrNonce(BytesToUint160(coinbase.Sender))
		states.UpdateBalance(BytesToUint160(coinbase.Recipient), Fixed64(coinbase.Amount)+totalFee, Addition)
	case pb.TransferAssetType:
		transfer := pl.(*pb.TransferAsset)
		states.UpdateBalance(BytesToUint160(transfer.Sender), Fixed64(transfer.Amount)+Fixed64(txn.UnsignedTx.Fee), Subtraction)
		states.IncrNonce(BytesToUint160(transfer.Sender))
		states.UpdateBalance(BytesToUint160(transfer.Recipient), Fixed64(transfer.Amount), Addition)

	case pb.RegisterNameType:
		fallthrough
	case pb.DeleteNameType:
		fallthrough
	case pb.SubscribeType:
		pg, err := txn.GetProgramHashes()
		if err != nil {
			return err
		}

		if err := states.UpdateBalance(pg[0], Fixed64(txn.UnsignedTx.Fee), Subtraction); err != nil {
			return err
		}
		states.IncrNonce(pg[0])

	case pb.GenerateIDType:
		genID := pl.(*pb.GenerateID)
		pg, err := txn.GetProgramHashes()
		if err != nil {
			return err
		}

		if err := states.UpdateBalance(pg[0], Fixed64(genID.RegistrationFee)+Fixed64(txn.UnsignedTx.Fee), Subtraction); err != nil {
			return err
		}
		states.IncrNonce(pg[0])
		states.UpdateID(pg[0], crypto.Sha256ZeroHash)

		donationAddress, err := ToScriptHash(config.DonationAddress)
		if err != nil {
			return err
		}
		states.UpdateBalance(donationAddress, Fixed64(genID.RegistrationFee), Addition)

	case pb.NanoPayType:
		nanoPay := pl.(*pb.NanoPay)
		pg, err := txn.GetProgramHashes()
		if err != nil {
			return err
		}

		addrRecipient := BytesToUint160(nanoPay.Recipient)
		nanoPayBalance, _, err := states.GetNanoPay(pg[0], addrRecipient, nanoPay.Nonce)
		if err != nil {
			return err
		}
		claimAmount := Fixed64(nanoPay.Amount) - nanoPayBalance
		if err := states.UpdateBalance(pg[0], claimAmount, Subtraction); err != nil {
			return err
		}
		if err := states.UpdateBalance(addrRecipient, claimAmount, Addition); err != nil {
			return err
		}
		states.SetNanoPay(pg[0], addrRecipient, nanoPay.Nonce, Fixed64(nanoPay.Amount), nanoPay.Height+nanoPay.Duration)
	}

	return nil
}

func (cs *ChainStore) GenerateStateRoot(b *block.Block, genesisBlockInitialized, needBeCommitted bool) (Uint256, error) {
	_, root, err := cs.generateStateRoot(b, genesisBlockInitialized, needBeCommitted)

	return root, err
}

func (cs *ChainStore) generateStateRoot(b *block.Block, genesisBlockInitialized, needBeCommitted bool) (*StateDB, Uint256, error) {
	stateRoot := EmptyUint256
	if genesisBlockInitialized {
		var err error
		stateRoot, err = cs.GetCurrentBlockStateRoot()
		if err != nil {
			return nil, EmptyUint256, err
		}
	}
	states, err := NewStateDB(stateRoot, NewTrieStore(cs.GetDatabase()))
	if err != nil {
		return nil, EmptyUint256, err
	}

	//process previous block
	height := b.Header.UnsignedHeader.Height
	if height == 0 {
		id := ComputeID(EmptyUint256, EmptyUint256, b.Header.UnsignedHeader.RandomBeacon[:config.RandomBeaconUniqueLength])

		pk, err := crypto.NewPubKeyFromBytes(b.Header.UnsignedHeader.Signer)
		if err != nil {
			return nil, EmptyUint256, err
		}
		contract, err := contract.CreateSignatureContract(pk)
		if err != nil {
			return nil, EmptyUint256, err
		}

		if err := states.UpdateID(contract.ProgramHash, id); err != nil {
			return nil, EmptyUint256, err
		}
	}

	if height > config.GenerateIDBlockDelay {
		prevBlock, err := cs.GetBlockByHeight(height - config.GenerateIDBlockDelay)
		if err != nil {
			return nil, EmptyUint256, err
		}

		preBlockHash := prevBlock.Hash()

		for _, txn := range prevBlock.Transactions {
			if txn.UnsignedTx.Payload.Type == pb.GenerateIDType {
				id := ComputeID(preBlockHash, txn.Hash(), b.Header.UnsignedHeader.RandomBeacon[:config.RandomBeaconUniqueLength])

				pg, err := txn.GetProgramHashes()
				if err != nil {
					return nil, EmptyUint256, err
				}

				if err := states.UpdateID(pg[0], id); err != nil {
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
		if err := cs.spendTransaction(states, b.Transactions[0], totalFee, true); err != nil {
			return nil, EmptyUint256, err
		}
		for _, txn := range b.Transactions[1:] {
			if err := cs.spendTransaction(states, txn, 0, false); err != nil {
				return nil, EmptyUint256, err
			}
		}
	} else {
		for _, txn := range b.Transactions {
			if err := cs.spendTransaction(states, txn, totalFee, false); err != nil {
				return nil, EmptyUint256, err
			}
		}

		states.CleanupNanoPay(b.Header.UnsignedHeader.Height)
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

func ComputeID(preBlockHash, txnHash Uint256, randomBeacon []byte) []byte {
	data := append(preBlockHash[:], txnHash[:]...)
	data = append(data, randomBeacon...)
	id := crypto.Sha256(data)
	return id
}
