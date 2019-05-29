package chain

import (
	"github.com/nknorg/nkn/chain/db"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	. "github.com/nknorg/nkn/pb"
	. "github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nnet/log"
)

func spendTransaction(states *db.StateDB, tx *Transaction, totalFee common.Fixed64, genesis bool) error {
	pl, err := Unpack(tx.UnsignedTx.Payload)
	if err != nil {
		log.Error("unpack payload error", err)
		return err
	}

	switch tx.UnsignedTx.Payload.Type {
	case CoinbaseType:
		coinbase := pl.(*Coinbase)
		accSender := states.GetOrNewAccount(common.BytesToUint160(coinbase.Sender))
		if !genesis {
			amountSender := accSender.GetBalance()
			donation, err := DefaultLedger.Store.GetDonation()
			if err != nil {
				log.Error("get donation from store err", err)
				return err
			}
			accSender.SetBalance(amountSender - donation.Amount)
		}
		nonceSender := accSender.GetNonce()
		accSender.SetNonce(nonceSender + 1)
		states.SetAccount(common.BytesToUint160(coinbase.Sender), accSender)

		acc := states.GetOrNewAccount(common.BytesToUint160(coinbase.Recipient))
		amount := acc.GetBalance()
		acc.SetBalance(amount + common.Fixed64(coinbase.Amount) + totalFee)
		states.SetAccount(common.BytesToUint160(coinbase.Recipient), acc)

	case TransferAssetType:
		transfer := pl.(*TransferAsset)
		accSender := states.GetOrNewAccount(common.BytesToUint160(transfer.Sender))
		amountSender := accSender.GetBalance()
		accSender.SetBalance(amountSender - common.Fixed64(transfer.Amount) - common.Fixed64(tx.UnsignedTx.Fee))
		nonceSender := accSender.GetNonce()
		accSender.SetNonce(nonceSender + 1)
		states.SetAccount(common.BytesToUint160(transfer.Sender), accSender)

		accRecipient := states.GetOrNewAccount(common.BytesToUint160(transfer.Recipient))
		amountRecipient := accRecipient.GetBalance()
		accRecipient.SetBalance(amountRecipient + common.Fixed64(transfer.Amount))
		states.SetAccount(common.BytesToUint160(transfer.Recipient), accRecipient)
	case RegisterNameType:
		fallthrough
	case DeleteNameType:
		fallthrough
	case SubscribeType:
		pg, _ := common.ToCodeHash(tx.Programs[0].Code)
		accSender := states.GetOrNewAccount(pg)
		amountSender := accSender.GetBalance()
		accSender.SetBalance(amountSender - common.Fixed64(tx.UnsignedTx.Fee))
		nonceSender := accSender.GetNonce()
		accSender.SetNonce(nonceSender + 1)
		states.SetAccount(pg, accSender)
	case GenerateIDType:
		genID := pl.(*GenerateID)
		pg, _ := common.ToCodeHash(tx.Programs[0].Code)
		accSender := states.GetOrNewAccount(pg)
		amountSender := accSender.GetBalance()
		accSender.SetBalance(amountSender - common.Fixed64(genID.RegistrationFee) - common.Fixed64(tx.UnsignedTx.Fee))
		nonceSender := accSender.GetNonce()
		accSender.SetNonce(nonceSender + 1)
		accSender.SetID(crypto.Sha256ZeroHash)
		states.SetAccount(pg, accSender)

		donationAddress, err := common.ToScriptHash(config.DonationAddress)
		if err != nil {
			return err
		}
		donationAccount := states.GetOrNewAccount(donationAddress)
		amount := donationAccount.GetBalance()
		donationAccount.SetBalance(amount + common.Fixed64(genID.RegistrationFee))
		states.SetAccount(donationAddress, donationAccount)

	}

	return nil
}

func GenerateStateRoot(txs []*Transaction, height uint32, randomBeacon []byte) common.Uint256 {

	root := DefaultLedger.Store.GetCurrentBlockStateRoot()
	states, _ := db.NewStateDB(root, db.NewTrieStore(DefaultLedger.Store.GetDatabase()))

	//process previous block
	if height > config.GenerateIDBlockDelay {
		prevBlock, err := DefaultLedger.Store.GetBlockByHeight(height - config.GenerateIDBlockDelay)
		if err != nil {
			return common.EmptyUint256
		}

		preBlockHash := prevBlock.Hash()

		for _, txn := range prevBlock.Transactions {
			if txn.UnsignedTx.Payload.Type == GenerateIDType {
				txnHash := txn.Hash()
				data := append(preBlockHash[:], txnHash[:]...)
				data = append(data, randomBeacon...)
				id := crypto.Sha256(data)

				pg, err := txn.GetProgramHashes()
				if err != nil {
					return common.EmptyUint256
				}
				accSender := states.GetOrNewAccount(pg[0])
				accSender.SetID(id)
				states.SetAccount(pg[0], accSender)
			}
		}
	}

	var totalFee common.Fixed64
	for _, tx := range txs {
		totalFee += common.Fixed64(tx.UnsignedTx.Fee)
	}

	for _, tx := range txs {
		spendTransaction(states, tx, totalFee, false)
	}

	return states.IntermediateRoot(true)
}

func GenesisStateRoot(store ILedgerStore, txs []*Transaction) common.Uint256 {
	root := common.EmptyUint256
	states, _ := db.NewStateDB(root, db.NewTrieStore(store.GetDatabase()))

	for _, tx := range txs {
		spendTransaction(states, tx, 0, true)
	}

	return states.IntermediateRoot(true)
}
