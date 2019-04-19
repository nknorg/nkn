package chain

import (
	"github.com/nknorg/nkn/chain/db"
	"github.com/nknorg/nkn/common"
	. "github.com/nknorg/nkn/pb"
	. "github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nnet/log"
)

func spendTransaction(states *db.StateDB, tx *Transaction, genesis bool) error {
	pl, err := Unpack(tx.UnsignedTx.Payload)
	if err != nil {
		log.Error("unpack payload error", err)
		return err
	}

	switch tx.UnsignedTx.Payload.Type {
	case CoinbaseType:
		coinbase := pl.(*Coinbase)
		if !genesis {
			accSender := states.GetOrNewAccount(common.BytesToUint160(coinbase.Sender))
			amountSender := accSender.GetBalance()
			donation, err := DefaultLedger.Store.GetDonation()
			if err != nil {
				log.Error("get donation from store err", err)
				return err
			}
			accSender.SetBalance(amountSender - donation.Amount)
			states.SetAccount(common.BytesToUint160(coinbase.Sender), accSender)
		}

		acc := states.GetOrNewAccount(common.BytesToUint160(coinbase.Recipient))
		amount := acc.GetBalance()
		acc.SetBalance(amount + common.Fixed64(coinbase.Amount))
		states.SetAccount(common.BytesToUint160(coinbase.Recipient), acc)

	case TransferAssetType:
		transfer := pl.(*TransferAsset)
		accSender := states.GetOrNewAccount(common.BytesToUint160(transfer.Sender))
		amountSender := accSender.GetBalance()
		accSender.SetBalance(amountSender - common.Fixed64(transfer.Amount))
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
		nonceSender := accSender.GetNonce()
		accSender.SetNonce(nonceSender + 1)
		states.SetAccount(pg, accSender)
	}

	return nil
}

func GenerateStateRoot(txs []*Transaction) common.Uint256 {

	root := DefaultLedger.Store.GetCurrentBlockStateRoot()
	states, _ := db.NewStateDB(root, db.NewTrieStore(DefaultLedger.Store.GetDatabase()))

	for _, tx := range txs {
		spendTransaction(states, tx, false)
	}

	return states.IntermediateRoot(true)
}

func GenesisStateRoot(store ILedgerStore, txs []*Transaction) common.Uint256 {
	root := common.EmptyUint256
	states, _ := db.NewStateDB(root, db.NewTrieStore(store.GetDatabase()))

	for _, tx := range txs {
		spendTransaction(states, tx, true)
	}

	return states.IntermediateRoot(true)
}
