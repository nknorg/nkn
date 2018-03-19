package node

import (
	"nkn-core/common"
	"nkn-core/common/config"
	"nkn-core/common/log"
	"nkn-core/core/ledger"
	. "nkn-core/errors"
	"fmt"
	"sync"
)

type TXNPool struct {
	sync.RWMutex
	txnCnt        uint64                                      // count
	txnList       map[common.Uint256]*ledger.Transaction // transaction which have been verifyed will put into this map
	issueSummary  map[common.Uint256]common.Fixed64           // transaction which pass the verify will summary the amout to this map
	inputUTXOList map[string]*ledger.Transaction         // transaction which pass the verify will add the UTXO to this map
}

func (this *TXNPool) init() {
	this.Lock()
	defer this.Unlock()
	this.txnCnt = 0
	this.inputUTXOList = make(map[string]*ledger.Transaction)
	this.issueSummary = make(map[common.Uint256]common.Fixed64)
	this.txnList = make(map[common.Uint256]*ledger.Transaction)
}

//append transaction to txnpool when check ok.
//1.check transaction. 2.check with ledger(db) 3.check with pool
func (this *TXNPool) AppendTxnPool(txn *ledger.Transaction) ErrCode {
	//verify transaction with Concurrency
	if errCode := ledger.VerifyTransaction(txn); errCode != ErrNoError {
		log.Info("Transaction verification failed", txn.Hash())
		return errCode
	}
	if errCode := ledger.VerifyTransactionWithLedger(txn, ledger.DefaultLedger); errCode != ErrNoError {
		log.Info("Transaction verification with ledger failed", txn.Hash())
		return errCode
	}
	//verify transaction by pool with lock
	if ok := this.verifyTransactionWithTxnPool(txn); !ok {
		return ErrSummaryAsset
	}
	//add the transaction to process scope
	this.addtxnList(txn)
	return ErrNoError
}

//get the transaction in txnpool
func (this *TXNPool) GetTxnPool(byCount bool) map[common.Uint256]*ledger.Transaction {
	this.RLock()
	count := config.Parameters.MaxTxInBlock
	if count <= 0 {
		byCount = false
	}
	if len(this.txnList) < count || !byCount {
		count = len(this.txnList)
	}
	var num int
	txnMap := make(map[common.Uint256]*ledger.Transaction, count)
	for txnId, tx := range this.txnList {
		txnMap[txnId] = tx
		num++
		if num >= count {
			break
		}
	}
	this.RUnlock()
	return txnMap
}

//clean the trasaction Pool with committed block.
func (this *TXNPool) CleanSubmittedTransactions(block *ledger.Block) error {
	this.cleanTransactionList(block.Transactions)
	this.cleanUTXOList(block.Transactions)
	return nil
}

//get the transaction by hash
func (this *TXNPool) GetTransaction(hash common.Uint256) *ledger.Transaction {
	this.RLock()
	defer this.RUnlock()
	return this.txnList[hash]
}

//verify transaction with txnpool
func (this *TXNPool) verifyTransactionWithTxnPool(txn *ledger.Transaction) bool {
	//check weather have duplicate UTXO input,if occurs duplicate, just keep the latest txn.
	ok, duplicateTxn := this.apendToUTXOPool(txn)
	if !ok && duplicateTxn != nil {
		log.Info(fmt.Sprintf("txn=%x duplicateTxn UTXO occurs with txn in pool=%x,keep the latest one.", txn.Hash(), duplicateTxn.Hash()))
		this.removeTransaction(duplicateTxn)
	}
	return true
}

//remove from associated map
func (this *TXNPool) removeTransaction(txn *ledger.Transaction) {
	//1.remove from txnList
	this.deltxnList(txn)
	//2.remove from UTXO list map
	result, err := txn.GetReference()
	if err != nil {
		log.Info(fmt.Sprintf("Transaction =%x not Exist in Pool when delete.", txn.Hash()))
		return
	}
	for UTXOTxInput, _ := range result {
		this.delInputUTXOList(UTXOTxInput)
	}
	transactionResult := txn.GetMergedAssetIDValueFromOutputs()
	for k, delta := range transactionResult {
		this.decrAssetIssueAmountSummary(k, delta)
	}
}

//check and add to utxo list pool
func (this *TXNPool) apendToUTXOPool(txn *ledger.Transaction) (bool, *ledger.Transaction) {
	reference, err := txn.GetReference()
	if err != nil {
		return false, nil
	}
	for k, _ := range reference {
		t := this.getInputUTXOList(k)
		if t != nil {
			return false, t
		}
		this.addInputUTXOList(txn, k)
	}
	return true, nil
}

//clean txnpool utxo map
func (this *TXNPool) cleanUTXOList(txs []*ledger.Transaction) {
	for _, txn := range txs {
		inputUtxos, _ := txn.GetReference()
		for Utxoinput, _ := range inputUtxos {
			this.delInputUTXOList(Utxoinput)
		}
	}
}

// clean the trasaction Pool with committed transactions.
func (this *TXNPool) cleanTransactionList(txns []*ledger.Transaction) error {
	cleaned := 0
	txnsNum := len(txns)
	for _, txn := range txns {
		if txn.TxType == ledger.BookKeeping {
			txnsNum = txnsNum - 1
			continue
		}
		if this.deltxnList(txn) {
			cleaned++
		}
	}
	if txnsNum != cleaned {
		log.Info(fmt.Sprintf("The Transactions num Unmatched. Expect %d, got %d .\n", txnsNum, cleaned))
	}
	log.Debug(fmt.Sprintf("[cleanTransactionList],transaction %d Requested, %d cleaned, Remains %d in TxPool", txnsNum, cleaned, this.GetTransactionCount()))
	return nil
}

func (this *TXNPool) addtxnList(txn *ledger.Transaction) bool {
	this.Lock()
	defer this.Unlock()
	txnHash := txn.Hash()
	if _, ok := this.txnList[txnHash]; ok {
		return false
	}
	this.txnList[txnHash] = txn
	return true
}

func (this *TXNPool) deltxnList(tx *ledger.Transaction) bool {
	this.Lock()
	defer this.Unlock()
	txHash := tx.Hash()
	if _, ok := this.txnList[txHash]; !ok {
		return false
	}
	delete(this.txnList, tx.Hash())
	return true
}

func (this *TXNPool) copytxnList() map[common.Uint256]*ledger.Transaction {
	this.RLock()
	defer this.RUnlock()
	txnMap := make(map[common.Uint256]*ledger.Transaction, len(this.txnList))
	for txnId, txn := range this.txnList {
		txnMap[txnId] = txn
	}
	return txnMap
}

func (this *TXNPool) GetTransactionCount() int {
	this.RLock()
	defer this.RUnlock()
	return len(this.txnList)
}

func (this *TXNPool) getInputUTXOList(input *ledger.UTXOTxInput) *ledger.Transaction {
	this.RLock()
	defer this.RUnlock()
	return this.inputUTXOList[input.ToString()]
}

func (this *TXNPool) addInputUTXOList(tx *ledger.Transaction, input *ledger.UTXOTxInput) bool {
	this.Lock()
	defer this.Unlock()
	id := input.ToString()
	_, ok := this.inputUTXOList[id]
	if ok {
		return false
	}
	this.inputUTXOList[id] = tx

	return true
}

func (this *TXNPool) delInputUTXOList(input *ledger.UTXOTxInput) bool {
	this.Lock()
	defer this.Unlock()
	id := input.ToString()
	_, ok := this.inputUTXOList[id]
	if !ok {
		return false
	}
	delete(this.inputUTXOList, id)
	return true
}

func (this *TXNPool) incrAssetIssueAmountSummary(assetId common.Uint256, delta common.Fixed64) {
	this.Lock()
	defer this.Unlock()
	this.issueSummary[assetId] = this.issueSummary[assetId] + delta
}

func (this *TXNPool) decrAssetIssueAmountSummary(assetId common.Uint256, delta common.Fixed64) {
	this.Lock()
	defer this.Unlock()
	amount, ok := this.issueSummary[assetId]
	if !ok {
		return
	}
	amount = amount - delta
	if amount < common.Fixed64(0) {
		amount = common.Fixed64(0)
	}
	this.issueSummary[assetId] = amount
}