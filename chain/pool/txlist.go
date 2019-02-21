package pool

import (
	"errors"
	"sync"

	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/pb"
	. "github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nnet/log"
)

var (
	ErrNonceSortedTxsEmpty = errors.New("Empty NonceSortedTxs")
	ErrNonceOutofRange     = errors.New("nonce is not in range")
)

// NonceSortedTxs store the txns that can be add into blockchain.
// The txns are sorted by nonce in Increasing order.
type NonceSortedTxs struct {
	mu      sync.RWMutex
	account common.Uint160
	txs     map[common.Uint256]*Transaction // txns belong to The same address
	idx     []common.Uint256                // the sequential tx hash list
	orphans map[common.Uint256]*Transaction // orphan txs cannot be added to ledger currently.
	cap     int                             // the capacity of the tx hash list
}

// NewNonceSortedTxs return a new NonceSortedTxs instance
func NewNonceSortedTxs(acc common.Uint160, length int) *NonceSortedTxs {
	return &NonceSortedTxs{
		account: acc,
		txs:     make(map[common.Uint256]*Transaction),
		idx:     make([]common.Uint256, 0),
		orphans: make(map[common.Uint256]*Transaction),
		cap:     length,
	}
}

func (nst *NonceSortedTxs) len() int {
	return len(nst.idx)
}

func (nst *NonceSortedTxs) Len() int {
	nst.mu.RLock()
	defer nst.mu.RUnlock()
	return nst.len()
}

func (nst *NonceSortedTxs) empty() bool {
	return nst.len() == 0
}

func (nst *NonceSortedTxs) Empty() bool {
	nst.mu.RLock()
	defer nst.mu.RUnlock()
	return nst.empty()
}

func (nst *NonceSortedTxs) full() bool {
	return nst.len() == nst.cap
}

func (nst *NonceSortedTxs) Full() bool {
	nst.mu.RLock()
	defer nst.mu.RUnlock()
	return nst.full()
}

func (nst *NonceSortedTxs) Push(tx *Transaction) error {
	nst.mu.Lock()
	defer nst.mu.Unlock()

	hash := tx.Hash()
	nst.idx = append(nst.idx, hash)
	nst.txs[hash] = tx

	return nil
}

func (nst *NonceSortedTxs) Pop() (*Transaction, error) {
	nst.mu.Lock()
	defer nst.mu.Unlock()

	if nst.empty() {
		return nil, ErrNonceSortedTxsEmpty
	}

	hash := nst.idx[0]
	nst.idx = nst.idx[1:]
	tx := nst.txs[hash]
	delete(nst.txs, hash)

	return tx, nil
}

func (nst *NonceSortedTxs) Seek() (*Transaction, error) {
	nst.mu.RLock()
	defer nst.mu.RUnlock()

	if nst.empty() {
		return nil, ErrNonceSortedTxsEmpty
	}

	return nst.txs[nst.idx[0]], nil
}

func (nst *NonceSortedTxs) getNonce(hash common.Uint256) uint64 {
	if tx, ok := nst.txs[hash]; ok {
		return tx.UnsignedTx.Nonce
	}

	panic("no such tx in NonceSortedTxs")
}

func (nst *NonceSortedTxs) Add(tx *Transaction) error {
	nst.mu.Lock()
	defer nst.mu.Unlock()

	if nst.empty() {
		return ErrNonceSortedTxsEmpty
	}

	txNonce := tx.UnsignedTx.Nonce
	if txNonce < nst.getNonce(nst.idx[0]) || txNonce > nst.getNonce(nst.idx[nst.len()-1]) {
		return ErrNonceOutofRange
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

	if nst.empty() {
		return nil, ErrNonceSortedTxsEmpty
	}

	if nonce < nst.getNonce(nst.idx[0]) || nonce > nst.getNonce(nst.idx[nst.len()-1]) {
		return nil, ErrNonceOutofRange
	}

	hash := nst.idx[nonce-nst.getNonce(nst.idx[0])]

	return nst.txs[hash], nil
}

func (nst *NonceSortedTxs) GetLatestNonce() (uint64, error) {
	nst.mu.RLock()
	defer nst.mu.RUnlock()

	if nst.empty() {
		return 0, ErrNonceSortedTxsEmpty
	}

	return nst.getNonce(nst.idx[nst.len()-1]), nil

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

func (nst *NonceSortedTxs) GetOrphanTxn(hash common.Uint256) *Transaction {
	if txn, ok := nst.orphans[hash]; ok {
		return txn
	}

	return nil
}

func (nst *NonceSortedTxs) AddOrphanTxn(txn *Transaction) error {
	nst.orphans[txn.Hash()] = txn

	return nil
}

func (nst *NonceSortedTxs) ProcessOrphans(handle func(tx *Transaction) error) error {
	for _, orphan := range nst.orphans {
		handle(orphan)
	}

	return nil
}

func (nst *NonceSortedTxs) CleanOrphans(txs []*Transaction) {
	nonce, err := nst.GetLatestNonce()
	if err != nil {
		nonce = chain.DefaultLedger.Store.GetNonce(nst.account)
	}

	for _, txn := range txs {
		hash := txn.Hash()
		if _, ok := nst.orphans[hash]; ok {
			delete(nst.orphans, hash)
		}
	}

	for hash, orphan := range nst.orphans {
		if orphan.UnsignedTx.Nonce <= nonce {
			delete(nst.orphans, hash)
		}

		if _, ok := nst.txs[hash]; ok {
			delete(nst.orphans, hash)
		}
	}

	//clean for payload type
}

func (nst *NonceSortedTxs) Dump() {
	nst.mu.RLock()
	defer nst.mu.RUnlock()
	addr, _ := nst.account.ToAddress()
	log.Error("account:", addr)
	log.Error("txs:", len(nst.txs))
	for h, tx := range nst.txs {
		log.Error(h.ToHexString(), ":", tx.UnsignedTx.Nonce)
	}
	log.Error("idx:", len(nst.idx))
	for _, h := range nst.idx {
		log.Error(h.ToHexString())
	}
	log.Error("orphans:", len(nst.orphans))
	for h, tx := range nst.orphans {
		log.Error(h.ToHexString(), ":", tx.UnsignedTx.Nonce)
	}
	log.Error("cap:", nst.cap)
}

//type FeeSortedTxns []*Transaction
