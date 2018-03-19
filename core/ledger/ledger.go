package ledger

import (
	. "nkn-core/common"
	"nkn-core/core/asset"
	. "nkn-core/errors"
)

var DefaultLedger *Ledger

type Ledger struct {
	Blockchain *Blockchain
	Store      ILedgerStore
}

//check weather the transaction contains the doubleSpend.
func (l *Ledger) IsDoubleSpend(Tx *Transaction) bool {
	return DefaultLedger.Store.IsDoubleSpend(Tx)
}

//Get the Asset from store.
func (l *Ledger) GetAsset(assetId Uint256) (*asset.Asset, error) {
	asset, err := l.Store.GetAsset(assetId)
	if err != nil {
		return nil, NewDetailErr(err, ErrNoCode, "[Ledger],GetAsset failed with assetId ="+assetId.ToString())
	}
	return asset, nil
}

//Get Block With Height.
func (l *Ledger) GetBlockWithHeight(height uint32) (*Block, error) {
	temp, err := l.Store.GetBlockHashByHeight(height)
	if err != nil {
		return nil, NewDetailErr(err, ErrNoCode, "[Ledger],GetBlockWithHeight failed with height="+string(height))
	}
	bk, err := DefaultLedger.Store.GetBlock(temp)
	if err != nil {
		return nil, NewDetailErr(err, ErrNoCode, "[Ledger],GetBlockWithHeight failed with hash="+temp.ToString())
	}
	return bk, nil
}

//Get block with block hash.
func (l *Ledger) GetBlockWithHash(hash Uint256) (*Block, error) {
	bk, err := l.Store.GetBlock(hash)
	if err != nil {
		return nil, NewDetailErr(err, ErrNoCode, "[Ledger],GetBlockWithHeight failed with hash="+hash.ToString())
	}
	return bk, nil
}

//BlockInLedger checks if the block existed in ledger
func (l *Ledger) BlockInLedger(hash Uint256) bool {
	_, err := l.Store.GetBlock(hash)
	if err != nil {
		return false
	}

	return true
}

//Get transaction with hash.
func (l *Ledger) GetTransactionWithHash(hash Uint256) (*Transaction, error) {
	tx, err := l.Store.GetTransaction(hash)
	if err != nil {
		return nil, NewDetailErr(err, ErrNoCode, "[Ledger],GetTransactionWithHash failed with hash="+hash.ToString())
	}
	return tx, nil
}

//Get local block chain height.
func (l *Ledger) GetLocalBlockChainHeight() uint32 {
	return l.Blockchain.BlockHeight
}
