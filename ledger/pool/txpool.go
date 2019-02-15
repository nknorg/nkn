package pool

import (
	"errors"
	"sync"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/ledger"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	. "github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nnet/log"
)

const (
	DefaultCap = 1024
)

type TxnPool2 struct {
	mu      sync.Mutex
	TxLists map[common.Uint160]*NonceSortedTxs
	ListCap int
	Orphans map[common.Uint256]*Transaction // Orphans limit?
}

func NewTxPool() *TxnPool2 {
	return &TxnPool2{
		TxLists: make(map[common.Uint160]*NonceSortedTxs),
		ListCap: DefaultCap,
		Orphans: make(map[common.Uint256]*Transaction),
	}
}

func (tp *TxnPool2) AppendTxnPool(txn *Transaction) error {
	if err := ledger.VerifyTransaction(txn); err != nil {
		log.Info("Transaction verification failed", txn.Hash(), err)
		return err
	}
	if err := ledger.VerifyTransactionWithLedger(txn); err != nil {
		log.Info("Transaction verification with ledger failed", txn.Hash(), err)
		return err
	}

	// get signature chain from commit transaction then add it to POR server
	if txn.UnsignedTx.Payload.Type == pb.CommitType {
		added, err := por.GetPorServer().AddSigChainFromTx(txn)
		if err != nil {
			return err
		}
		if !added {
			return nil
		}
	}

	tp.mu.Lock()
	defer tp.mu.Unlock()
	//TODO 1. process all Orphens

	// 2. process txn
	hash := txn.Hash()
	sender, _ := common.ToCodeHash(txn.Programs[0].Code)

	if _, ok := tp.TxLists[sender]; !ok {
		tp.TxLists[sender] = NewNonceSortedTxs()
	}

	list := tp.TxLists[sender]
	if list.ExistTx(hash) {
		return errors.New("exist")
	}

	//replace
	if _, err := list.Get(txn.UnsignedTx.Nonce); err == nil {
		return list.Add(txn)
	}

	if !list.Empty() {
		if list.Len() >= tp.ListCap {
			return errors.New("full")
		}

		preNonce, _ := list.GetLatestNonce()

		//TODO a new function is needed to append tx.
		// check total account in it
		if txn.UnsignedTx.Nonce == preNonce+1 {
			//TODO process sender Orphans
			return list.Push(txn)
		}
	} else {
		// compare with DB
		preNonce := ledger.DefaultLedger.Store.GetNonce(sender)
		if txn.UnsignedTx.Nonce == preNonce+1 {
			//TODO process sender Orphans
			return list.Push(txn)
		}
	}

	// 3. add to orphans
	if _, ok := tp.Orphans[hash]; ok {
		return errors.New("Exist")
	}

	tp.Orphans[hash] = txn

	return nil
}

func (tp *TxnPool2) GetAllTransactions() map[common.Uint256]*Transaction {
	//TODO
	return nil
}
func (tp *TxnPool2) GetTransaction(hash common.Uint256) *Transaction {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	for _, list := range tp.TxLists {
		if list.ExistTx(hash) {
			return list.txs[hash]
		}
	}

	if _, ok := tp.Orphans[hash]; ok {
		return tp.Orphans[hash]
	}

	return nil
}

func (tp *TxnPool2) GetTxsFromPool() []*Transaction {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	txs := make([]*Transaction, 0)
	for _, list := range tp.TxLists {
		if tx, err := list.Seek(); err == nil {
			txs = append(txs, tx)
		}
	}

	return txs
}

func (tp *TxnPool2) CleanSubmittedTransactions(txns []*Transaction) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// clean submitted txs
	for _, txn := range txns {
		sender, _ := common.ToCodeHash(txn.Programs[0].Code)
		nonce := txn.UnsignedTx.Nonce

		if list, ok := tp.TxLists[sender]; ok {
			if _, err := list.Get(nonce); err == nil {
				for i := 0; uint64(i) <= nonce-list.getNonce(list.idx[0]); i++ {
					list.Pop()
				}
			}
		}
	}

	// TODO clean invalid txs

	return nil
}
