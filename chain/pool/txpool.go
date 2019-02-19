package pool

import (
	"errors"
	"sync"

	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/common"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	. "github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nnet/log"
)

const (
	DefaultCap = 1024
)

type TxnPool struct {
	mu      sync.Mutex
	TxLists map[common.Uint160]*NonceSortedTxs
	ListCap int
	Orphans map[common.Uint256]*Transaction // Orphans limit?
}

func NewTxPool() *TxnPool {
	return &TxnPool{
		TxLists: make(map[common.Uint160]*NonceSortedTxs),
		ListCap: DefaultCap,
		Orphans: make(map[common.Uint256]*Transaction),
	}
}

func (tp *TxnPool) processTx(txn *Transaction) error {
}

func (tp *TxnPool) AppendTxnPool(txn *Transaction) ErrCode {
	if err := tp.appendTxnPool(txn); err != nil {
		return ErrNoCode
	}

	tp.Dump()
	return ErrNoError
}

func (tp *TxnPool) appendTxnPool(txn *Transaction) error {
	if err := chain.VerifyTransaction(txn); err != nil {
		log.Info("Transaction verification failed", txn.Hash(), err)
		return err
	}
	if err := chain.VerifyTransactionWithLedger(txn); err != nil {
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
	for hash, tx := range tp.Orphans {
	}

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
		expectNonce := chain.DefaultLedger.Store.GetNonce(sender)
		if txn.UnsignedTx.Nonce == expectNonce {
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

func (tp *TxnPool) GetAllTransactions() map[common.Uint256]*Transaction {
	//TODO
	return nil
}
func (tp *TxnPool) GetTransaction(hash common.Uint256) *Transaction {
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

func (tp *TxnPool) getTxsFromPool() []*Transaction {
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

func (tp *TxnPool) CleanSubmittedTransactions(txns []*Transaction) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// clean submitted txs
	for _, txn := range txns {
		if txn.UnsignedTx.Payload.Type == pb.CoinbaseType {
			continue
		}
		sender, _ := common.ToCodeHash(txn.Programs[0].Code)
		txNonce := txn.UnsignedTx.Nonce

		if list, ok := tp.TxLists[sender]; ok {
			if _, err := list.Get(txNonce); err == nil {
				nonce := list.getNonce(list.idx[0])
				for i := 0; uint64(i) <= txNonce-nonce; i++ {
					list.Pop()
				}
			}
		}
	}

	// TODO clean invalid txs

	return nil
}

func (tp *TxnPool) GetTxnByCount(num int, hash common.Uint256) (map[common.Uint256]*Transaction, error) {
	txmap := make(map[common.Uint256]*Transaction)
	txs := tp.getTxsFromPool()
	for _, tx := range txs {
		txmap[tx.Hash()] = tx
	}

	return txmap, nil
}

func (tp *TxnPool) Dump() {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	for addr, list := range tp.TxLists {
		address, _ := addr.ToAddress()
		log.Error("-------", address, list.Len())
	}
	log.Error("-------", tp.ListCap)
	log.Error("-------", tp.Orphans, len(tp.Orphans))
}
