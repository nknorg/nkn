package pool

import (
	"errors"
	"sync"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/pb"
	. "github.com/nknorg/nkn/transaction"
)

type NonceSortedTxs struct {
	mu  sync.RWMutex
	txs map[common.Uint256]*Transaction
	idx []common.Uint256
}

func NewNonceSortedTxs() *NonceSortedTxs {
	return &NonceSortedTxs{
		txs: make(map[common.Uint256]*Transaction),
		idx: make([]common.Uint256, 0),
	}
}

func (nst *NonceSortedTxs) Reset() error {
	nst = NewNonceSortedTxs()
	return nil
}

func (nst *NonceSortedTxs) Len() int {
	return len(nst.idx)
}

func (nst *NonceSortedTxs) Empty() bool {
	return nst.Len() == 0
}

func (nst *NonceSortedTxs) Push(tx *Transaction) error {
	nst.mu.Lock()
	defer nst.mu.Unlock()

	//TODO compare with DB
	if nst.Empty() && nst.txs[nst.idx[nst.Len()-1]].UnsignedTx.Nonce+1 != tx.UnsignedTx.Nonce {
		return errors.New("nonce is not continuous")
	}

	hash := tx.Hash()
	nst.idx = append(nst.idx, hash)
	nst.txs[hash] = tx

	return nil
}

func (nst *NonceSortedTxs) Pop() (*Transaction, error) {
	nst.mu.Lock()
	defer nst.mu.Unlock()

	if nst.Empty() {
		return nil, errors.New("Empty")
	}

	hash := nst.idx[0]
	tx := nst.txs[hash]
	nst.idx = nst.idx[1:]
	delete(nst.txs, hash)

	return tx, nil
}

func (nst *NonceSortedTxs) Seek() (*Transaction, error) {
	nst.mu.Lock()
	defer nst.mu.Unlock()

	if nst.Empty() {
		return nil, errors.New("Empty")
	}

	return nst.txs[nst.idx[0]], nil
}

func (nst *NonceSortedTxs) getNonce(hash common.Uint256) uint64 {
	if tx, ok := nst.txs[hash]; ok {
		return tx.UnsignedTx.Nonce
	}

	panic("get tx error")
}

func (nst *NonceSortedTxs) Add(tx *Transaction) error {
	nst.mu.Lock()
	defer nst.mu.Unlock()

	if nst.Empty() {
		return errors.New("Empty")
	}

	txNonce := tx.UnsignedTx.Nonce
	if txNonce < nst.getNonce(nst.idx[0]) || txNonce > nst.getNonce(nst.idx[nst.Len()-1]) {
		return errors.New("nonce errror")
	}

	origHash := nst.idx[txNonce-nst.getNonce(nst.idx[0])]
	nst.idx[txNonce-nst.getNonce(nst.idx[0])] = tx.Hash()
	delete(nst.txs, origHash)
	nst.txs[tx.Hash()] = tx

	return nil
}

func (nst *NonceSortedTxs) Get(nonce uint64) (*Transaction, error) {
	nst.mu.RLock()
	defer nst.mu.RUnlock()

	if nst.Empty() {
		return nil, errors.New("Empty")
	}

	if nonce < nst.getNonce(nst.idx[0]) || nonce > nst.getNonce(nst.idx[nst.Len()-1]) {
		return nil, errors.New("nonce errror")
	}

	hash := nst.idx[nonce-nst.getNonce(nst.idx[0])]
	return nst.txs[hash], nil
}

func (nst *NonceSortedTxs) GetLatestNonce() (uint64, error) {
	nst.mu.RLock()
	defer nst.mu.RUnlock()

	if nst.Empty() {
		//TODO Get from DB
		return 0, errors.New("Empty")
	}

	return nst.getNonce(nst.idx[nst.Len()-1]), nil

}

func (nst *NonceSortedTxs) ExistTx(hash common.Uint256) bool {
	nst.mu.RLock()
	defer nst.mu.RUnlock()

	if _, ok := nst.txs[hash]; ok {
		return true
	}

	return false
}

func (nst *NonceSortedTxs) Totality() common.Fixed64 {
	nst.mu.RLock()
	defer nst.mu.RUnlock()

	var amount common.Fixed64
	for _, tx := range nst.txs {
		if tx.UnsignedTx.Payload.Type == pb.TransferAssetType {
			pl, _ := Unpack(tx.UnsignedTx.Payload)
			transfer := pl.(*pb.TransferAsset)
			amount += common.Fixed64(transfer.Amount)
		}
		//TODO add fee
	}

	return amount
}

//TODO add fee
//type FeeSortedTxs []*transaction.Transaction
