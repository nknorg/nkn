package pool

import (
	"fmt"
	"sync"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	. "github.com/nknorg/nkn/core/transaction"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/log"
)

const (
	ExclusivedSigchainHeight = 3
)

type TransactionMap map[common.Uint256]*Transaction

func (iterable TransactionMap) Iterate(handler func(item *Transaction) ErrCode) ErrCode {
	for _, item := range iterable {
		result := handler(item)
		if result != ErrNoError {
			return result
		}
	}

	return ErrNoError
}

type TxnPool struct {
	sync.RWMutex
	txnList map[common.Uint256]*Transaction // transaction which have been verified will put into this map
}

func NewTxnPool() *TxnPool {
	return &TxnPool{
		txnList: make(map[common.Uint256]*Transaction),
	}
}

//append transaction to txnpool when check ok.
//1.check transaction. 2.check with ledger(db) 3.check with pool
func (tp *TxnPool) AppendTxnPool(txn *Transaction) ErrCode {
	txnHash := txn.Hash()
	//verify transaction with Concurrency
	if errCode := VerifyTransaction(txn); errCode != ErrNoError {
		log.Infof("Transaction verification failed: %s", txnHash.ToHexString())
		return errCode
	}
	if errCode := VerifyTransactionWithLedger(txn); errCode != ErrNoError {
		log.Infof("Transaction verification with ledger failed: %s", txnHash.ToHexString())
		return errCode
	}

	// get signature chain from commit transaction then add it to POR server
	if txn.TxType == Commit {
		added, err := por.GetPorServer().AddSigChainFromTx(txn, ledger.DefaultLedger.Store.GetHeight())
		if err != nil {
			log.Infof("Add sigchain from transaction error: %v", err)
			return ErrerCode(NewDetailErr(err, ErrNoCode, err.Error()))
		}
		if !added {
			return ErrNonOptimalSigChain
		}
		return ErrNoError
	}

	//add the transaction to process scope
	if tp.addtxnList(txn) {
		// Check duplicate UTXO reference after append successful
		if errCode := VerifyTransactionWithBlock(TransactionMap(tp.txnList)); errCode != ErrNoError {
			log.Info("Transaction verification with block failed", txn.Hash())
			tp.deltxnList(txn) // Revert previous append action
			return errCode
		}
	} else {
		return ErrDuplicatedTx // Don't broadcast this txn if hash duplicated
	}

	return ErrNoError
}

func (tp *TxnPool) GetTxnByCount(num int) (map[common.Uint256]*Transaction, error) {
	tp.RLock()
	defer tp.RUnlock()

	n := len(tp.txnList)
	if num < n {
		n = num
	}

	i := 0
	txns := make(map[common.Uint256]*Transaction, n)

	//TODO sort transaction list
	for hash, txn := range tp.txnList {
		txns[hash] = txn
		i++
		if i >= n {
			break
		}
	}

	return txns, nil
}

//clean the trasaction Pool with committed block.
func (tp *TxnPool) CleanSubmittedTransactions(txns []*Transaction) error {
	tp.cleanTransactionList(txns)
	return nil
}

//get the transaction by hash
func (tp *TxnPool) GetTransaction(hash common.Uint256) *Transaction {
	tp.RLock()
	defer tp.RUnlock()
	return tp.txnList[hash]
}

func (tp *TxnPool) GetAllTransactions() map[common.Uint256]*Transaction {
	tp.RLock()
	defer tp.RUnlock()

	txns := make(map[common.Uint256]*Transaction, len(tp.txnList))
	for hash, txn := range tp.txnList {
		txns[hash] = txn
	}

	return txns
}

// clean the trasaction Pool with committed transactions.
func (tp *TxnPool) cleanTransactionList(txns []*Transaction) error {
	cleaned := 0
	txnsNum := len(txns)
	for _, txn := range txns {
		if txn.TxType == Coinbase {
			txnsNum = txnsNum - 1
			continue
		}
		if tp.deltxnList(txn) {
			cleaned++
		}
	}
	log.Debug(fmt.Sprintf("transaction pool cleaning, requested: %d, cleaned: %d, remains %d", txnsNum, cleaned, tp.GetTransactionCount()))
	return nil
}

func (tp *TxnPool) addtxnList(txn *Transaction) bool {
	tp.Lock()
	defer tp.Unlock()
	txnHash := txn.Hash()
	if _, ok := tp.txnList[txnHash]; ok {
		return false
	}
	tp.txnList[txnHash] = txn
	return true
}

func (tp *TxnPool) deltxnList(tx *Transaction) bool {
	tp.Lock()
	defer tp.Unlock()
	txHash := tx.Hash()
	if _, ok := tp.txnList[txHash]; !ok {
		return false
	}
	delete(tp.txnList, tx.Hash())
	return true
}

func (tp *TxnPool) GetTransactionCount() int {
	tp.RLock()
	defer tp.RUnlock()
	return len(tp.txnList)
}

func isHashExist(hash common.Uint256, hashSet []common.Uint256) bool {
	for _, h := range hashSet {
		if h.CompareTo(hash) == 0 {
			return true
		}
	}

	return false
}
