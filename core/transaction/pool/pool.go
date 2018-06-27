package pool

import (
	"fmt"
	"sync"

	"github.com/nknorg/nkn/common"
	. "github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/log"
)

type TxnPool struct {
	sync.RWMutex
	txnCnt        uint64                            // transaction count
	txnList       map[common.Uint256]*Transaction   // transaction which have been verified will put into this map
	issueSummary  map[common.Uint256]common.Fixed64 // transaction which pass the verify will summary the amout to this map
	inputUTXOList map[string]*Transaction           // transaction which pass the verify will add the UTXO to this map
}

func NewTxnPool() *TxnPool {
	return &TxnPool{
		txnCnt:        0,
		inputUTXOList: make(map[string]*Transaction),
		issueSummary:  make(map[common.Uint256]common.Fixed64),
		txnList:       make(map[common.Uint256]*Transaction),
	}
}

//append transaction to txnpool when check ok.
//1.check transaction. 2.check with ledger(db) 3.check with pool
func (tp *TxnPool) AppendTxnPool(txn *Transaction) ErrCode {
	//verify transaction with Concurrency
	if errCode := VerifyTransaction(txn); errCode != ErrNoError {
		log.Info("Transaction verification failed", txn.Hash())
		return errCode
	}
	if errCode := VerifyTransactionWithLedger(txn); errCode != ErrNoError {
		log.Info("Transaction verification with ledger failed", txn.Hash())
		return errCode
	}
	//verify transaction by pool with lock
	if ok := tp.verifyTransactionWithTxnPool(txn); !ok {
		return ErrSummaryAsset
	}
	//add the transaction to process scope
	tp.addtxnList(txn)

	// get signature chain from commit transaction then add it to POR server
	if txn.TxType == Commit {
		por.GetPorServer().AddSigChainFromTx(txn)
	}

	return ErrNoError
}

func (tp *TxnPool) GetTxnByCount(num int) map[common.Uint256]*Transaction {
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

	return txns
}

//clean the trasaction Pool with committed block.
func (tp *TxnPool) CleanSubmittedTransactions(txns []*Transaction) error {
	tp.cleanTransactionList(txns)
	tp.cleanUTXOList(txns)
	tp.cleanIssueSummary(txns)
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

//verify transaction with txnpool
func (tp *TxnPool) verifyTransactionWithTxnPool(txn *Transaction) bool {
	//check weather have duplicate UTXO input,if occurs duplicate, just keep the latest txn.
	ok, duplicateTxn := tp.apendToUTXOPool(txn)
	if !ok && duplicateTxn != nil {
		log.Info(fmt.Sprintf("txn=%x duplicateTxn UTXO occurs with txn in pool=%x,keep the latest one.", txn.Hash(), duplicateTxn.Hash()))
		tp.removeTransaction(duplicateTxn)
	}
	//check issue transaction weather occur exceed issue range.
	if ok := tp.summaryAssetIssueAmount(txn); !ok {
		log.Info(fmt.Sprintf("Check summary Asset Issue Amount failed with txn=%x", txn.Hash()))
		tp.removeTransaction(txn)
		return false
	}
	return true
}

//remove from associated map
func (tp *TxnPool) removeTransaction(txn *Transaction) {
	//1.remove from txnList
	tp.deltxnList(txn)
	//2.remove from UTXO list map
	result, err := txn.GetReference()
	if err != nil {
		log.Info(fmt.Sprintf("Transaction =%x not Exist in Pool when delete.", txn.Hash()))
		return
	}
	for input := range result {
		tp.delInputUTXOList(input)
	}
	//3.remove From Asset Issue Summary map
	if txn.TxType != IssueAsset {
		return
	}
	transactionResult := txn.GetMergedAssetIDValueFromOutputs()
	for k, delta := range transactionResult {
		tp.decrAssetIssueAmountSummary(k, delta)
	}
}

//check and add to utxo list pool
func (tp *TxnPool) apendToUTXOPool(txn *Transaction) (bool, *Transaction) {
	reference, err := txn.GetReference()
	if err != nil {
		return false, nil
	}
	for k, _ := range reference {
		t := tp.getInputUTXOList(k)
		if t != nil {
			return false, t
		}
		tp.addInputUTXOList(txn, k)
	}
	return true, nil
}

//clean txnpool utxo map
func (tp *TxnPool) cleanUTXOList(txs []*Transaction) {
	for _, txn := range txs {
		inputUtxos, _ := txn.GetReference()
		for Utxoinput, _ := range inputUtxos {
			tp.delInputUTXOList(Utxoinput)
		}
	}
}

//check and summary to issue amount Pool
func (tp *TxnPool) summaryAssetIssueAmount(txn *Transaction) bool {
	if txn.TxType != IssueAsset {
		return true
	}
	transactionResult := txn.GetMergedAssetIDValueFromOutputs()
	for k, delta := range transactionResult {
		//update the amount in txnPool
		tp.incrAssetIssueAmountSummary(k, delta)

		//Check weather occur exceed the amount when RegisterAsseted
		//1. Get the Asset amount when RegisterAsseted.
		txn, err := TxStore.GetTransaction(k)
		if err != nil {
			return false
		}
		if txn.TxType != RegisterAsset {
			return false
		}
		AssetReg := txn.Payload.(*payload.RegisterAsset)

		//2. Get the amount has been issued of tp assetID
		var quantity_issued common.Fixed64
		if AssetReg.Amount < common.Fixed64(0) {
			continue
		} else {
			quantity_issued, err = TxStore.GetQuantityIssued(k)
			if err != nil {
				return false
			}
		}

		//3. calc weather out off the amount when Registed.
		//AssetReg.Amount : amount when RegisterAsset of tp assedID
		//quantity_issued : amount has been issued of tp assedID
		//txnPool.issueSummary[k] : amount in transactionPool of tp assedID
		if AssetReg.Amount-quantity_issued < tp.getAssetIssueAmount(k) {
			return false
		}
	}
	return true
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

func (tp *TxnPool) copytxnList() map[common.Uint256]*Transaction {
	tp.RLock()
	defer tp.RUnlock()
	txnMap := make(map[common.Uint256]*Transaction, len(tp.txnList))
	for txnId, txn := range tp.txnList {
		txnMap[txnId] = txn
	}
	return txnMap
}

func (tp *TxnPool) GetTransactionCount() int {
	tp.RLock()
	defer tp.RUnlock()
	return len(tp.txnList)
}

func (tp *TxnPool) getInputUTXOList(input *TxnInput) *Transaction {
	tp.RLock()
	defer tp.RUnlock()
	return tp.inputUTXOList[input.ToString()]
}

func (tp *TxnPool) addInputUTXOList(tx *Transaction, input *TxnInput) bool {
	tp.Lock()
	defer tp.Unlock()
	id := input.ToString()
	_, ok := tp.inputUTXOList[id]
	if ok {
		return false
	}
	tp.inputUTXOList[id] = tx

	return true
}

func (tp *TxnPool) delInputUTXOList(input *TxnInput) bool {
	tp.Lock()
	defer tp.Unlock()
	id := input.ToString()
	_, ok := tp.inputUTXOList[id]
	if !ok {
		return false
	}
	delete(tp.inputUTXOList, id)
	return true
}

func (tp *TxnPool) incrAssetIssueAmountSummary(assetId common.Uint256, delta common.Fixed64) {
	tp.Lock()
	defer tp.Unlock()
	tp.issueSummary[assetId] = tp.issueSummary[assetId] + delta
}

func (tp *TxnPool) decrAssetIssueAmountSummary(assetId common.Uint256, delta common.Fixed64) {
	tp.Lock()
	defer tp.Unlock()
	amount, ok := tp.issueSummary[assetId]
	if !ok {
		return
	}
	amount = amount - delta
	if amount < common.Fixed64(0) {
		amount = common.Fixed64(0)
	}
	tp.issueSummary[assetId] = amount
}

func (tp *TxnPool) cleanIssueSummary(txs []*Transaction) {
	for _, v := range txs {
		if v.TxType == IssueAsset {
			transactionResult := v.GetMergedAssetIDValueFromOutputs()
			for k, delta := range transactionResult {
				tp.decrAssetIssueAmountSummary(k, delta)
			}
		}
	}
}

func (tp *TxnPool) getAssetIssueAmount(assetId common.Uint256) common.Fixed64 {
	tp.RLock()
	defer tp.RUnlock()
	return tp.issueSummary[assetId]
}
