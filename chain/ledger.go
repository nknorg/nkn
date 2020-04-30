package chain

import (
	"errors"

	"github.com/nknorg/nkn/block"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/transaction"
)

var DefaultLedger *Ledger

type Ledger struct {
	Blockchain *Blockchain
	Store      ILedgerStore
}

// double spend checking for transaction
func (l *Ledger) IsDoubleSpend(Tx *transaction.Transaction) bool {
	return DefaultLedger.Store.IsDoubleSpend(Tx)
}

// get the default ledger
func GetDefaultLedger() (*Ledger, error) {
	if DefaultLedger == nil {
		return nil, errors.New("[Ledger] GetDefaultLedger failed, DefaultLedger not Exist.")
	}
	return DefaultLedger, nil
}

//Get the Asset from store.
//func (l *Ledger) GetAsset(assetId Uint256) (*asset.Asset, error) {
//	asset, err := l.Store.GetAsset(assetId)
//	if err != nil {
//		return nil, NewDetailErr(err, ErrNoCode, "[Ledger],GetAsset failed with assetId ="+assetId.ToString())
//	}
//	return asset, nil
//}

//Get Block With Height.
func (l *Ledger) GetBlockWithHeight(height uint32) (*block.Block, error) {
	temp, err := l.Store.GetBlockHash(height)
	if err != nil {
		return nil, errors.New("[Ledger] GetBlockWithHeight failed with height=" + string(height))
	}
	bk, err := DefaultLedger.Store.GetBlock(temp)
	if err != nil {
		return nil, errors.New("[Ledger] GetBlockWithHeight failed with hash=" + temp.ToString())
	}
	return bk, nil
}

//Get block with block hash.
func (l *Ledger) GetBlockWithHash(hash Uint256) (*block.Block, error) {
	bk, err := l.Store.GetBlock(hash)
	if err != nil {
		return nil, errors.New("[Ledger] GetBlockWithHeight failed with hash=" + hash.ToString())
	}
	return bk, nil
}

//BlockInLedger checks if the block existed in ledger
func (l *Ledger) BlockInLedger(hash Uint256) bool {
	return l.Store.IsBlockInStore(hash)
}

//Get transaction with hash.
func (l *Ledger) GetTransactionWithHash(hash Uint256) (*transaction.Transaction, error) {
	tx, err := l.Store.GetTransaction(hash)
	if err != nil {
		return nil, errors.New("[Ledger] GetTransactionWithHash failed with hash=" + hash.ToString())
	}
	return tx, nil
}

//Get local block chain height.
func (l *Ledger) GetLocalBlockChainHeight() uint32 {
	return l.Blockchain.BlockHeight
}
