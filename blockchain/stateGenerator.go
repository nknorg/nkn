package blockchain

import (
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/db"
	"github.com/nknorg/nkn/types"
)

func spendTransaction(states *db.StateDB, tx *types.Transaction) error {
	pl, err := types.Unpack(tx.UnsignedTx.Payload)
	if err != nil {
		return err
	}

	switch tx.UnsignedTx.Payload.Type {
	case types.CoinbaseType:
		coinbase := pl.(*types.Coinbase)
		acc := states.GetOrNewAccount(common.BytesToUint160(coinbase.Recipient))
		amount := acc.GetBalance()
		acc.SetBalance(amount + common.Fixed64(coinbase.Amount))
		states.SetAccount(common.BytesToUint160(coinbase.Recipient), acc)

	case types.TransferAssetType:
		transfer := pl.(*types.TransferAsset)
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

	}

	return nil
}

func GenerateStateRoot(txs []*types.Transaction) common.Uint256 {

	root := DefaultLedger.Store.GetCurrentBlockStateRoot()
	states, _ := db.NewStateDB(root, db.NewTrieStore(DefaultLedger.Store.GetDatabase()))

	for _, tx := range txs {
		spendTransaction(states, tx)
	}

	return states.IntermediateRoot(true)
}

func GenesisStateRoot(store ILedgerStore, txs []*types.Transaction) common.Uint256 {
	root := common.EmptyUint256
	states, _ := db.NewStateDB(root, db.NewTrieStore(store.GetDatabase()))

	for _, tx := range txs {
		spendTransaction(states, tx)
	}

	return states.IntermediateRoot(true)
}
