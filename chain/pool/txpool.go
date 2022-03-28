package pool

import (
	"container/heap"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/chain/txvalidator"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/transaction"
	"github.com/nknorg/nkn/v2/util/log"
)

var (
	ErrDuplicatedTx      = errors.New("duplicate transaction check failed")
	ErrRejectLowPriority = errors.New("txpool full, rejecting transaction with low priority")
	ErrNanoPayReplace    = errors.New("cannot replace nano pay txn with lower amount or expiration")
)

type nanoPay struct {
	sender    string
	recipient string
	nonce     uint64
}

func nanoPayKey(sender, recipient string, nonce uint64) nanoPay {
	return nanoPay{sender, recipient, nonce}
}

// TxnPool is a list of txns that need to by add to ledger sent by user.
type TxnPool struct {
	TxLists              sync.Map // NonceSortedTxs instance to store user's account.
	TxMap                sync.Map
	TxShortHashMap       sync.Map
	NanoPayTxs           sync.Map // tx with nano pay type.
	blockValidationState *chain.BlockValidationState
	txnCount             int32
	txnSize              int64

	sync.RWMutex
	lastDroppedTxn *transaction.Transaction
}

func NewTxPool() *TxnPool {
	tp := &TxnPool{
		blockValidationState: chain.NewBlockValidationState(),
		txnCount:             0,
	}

	go func() {
		for {
			tp.DropTxns()
			time.Sleep(config.TxPoolCleanupInterval)
		}
	}()

	return tp
}

func isTxPoolFull(txnCount int32, txnSize int64) bool {
	if config.Parameters.TxPoolTotalTxCap > 0 && txnCount > int32(config.Parameters.TxPoolTotalTxCap) {
		return true
	}
	if config.Parameters.TxPoolMaxMemorySize > 0 && txnSize > int64(config.Parameters.TxPoolMaxMemorySize)*1024*1024 {
		return true
	}
	return false
}

func (tp *TxnPool) getLastDroppedTxn() *transaction.Transaction {
	tp.RLock()
	defer tp.RUnlock()
	return tp.lastDroppedTxn
}

func (tp *TxnPool) setLastDroppedTxn(txn *transaction.Transaction) {
	tp.Lock()
	defer tp.Unlock()
	tp.lastDroppedTxn = txn
}

func (tp *TxnPool) DropTxns() {
	currentTxnCount := atomic.LoadInt32(&tp.txnCount)
	currentTxnSize := atomic.LoadInt64(&tp.txnSize)

	if !isTxPoolFull(currentTxnCount, currentTxnSize) {
		log.Infof("DropTxns: %v txns (%v bytes) in txpool, no need to drop", currentTxnCount, currentTxnSize)
		tp.setLastDroppedTxn(nil)
		return
	}

	log.Infof("DropTxns: %v txns (%v bytes) in txpool, need to drop txns", currentTxnCount, currentTxnSize)

	txnsDropped := make([]*transaction.Transaction, 0)
	dropList := make([]*transaction.Transaction, 0)

	tp.TxLists.Range(func(_, v interface{}) bool {
		if list, ok := v.(*NonceSortedTxs); ok {
			if listSize := list.Len(); listSize != 0 {
				txn, err := list.GetLatestTxn()
				if err == nil {
					dropList = append(dropList, txn)
				}
			}
		}

		return true
	})

	tp.NanoPayTxs.Range(func(_, v interface{}) bool {
		dropList = append(dropList, v.(*transaction.Transaction))
		return true
	})

	heap.Init(transaction.DefaultHeap(dropList))

	for {
		if len(dropList) == 0 {
			break
		}

		txn := heap.Pop(transaction.DefaultHeap(dropList)).(*transaction.Transaction)

		switch txn.UnsignedTx.Payload.Type {
		case pb.PayloadType_NANO_PAY_TYPE:
			payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
			if err != nil {
				continue
			}
			npPayload := payload.(*pb.NanoPay)
			key := nanoPayKey(string(npPayload.Sender), string(npPayload.Recipient), npPayload.Id)
			if _, ok := tp.NanoPayTxs.Load(key); !ok {
				continue
			}
			tp.NanoPayTxs.Delete(key)
		default:
			account, err := txn.GetProgramHashes()
			if err != nil {
				continue
			}

			v, ok := tp.TxLists.Load(account[0])
			if !ok {
				continue
			}

			list, ok := v.(*NonceSortedTxs)
			if !ok {
				continue
			}

			_, dropped, err := list.Drop(txn.Hash())
			if err != nil {
				continue
			}

			if list.Len() > 0 {
				nextTxn, err := list.GetLatestTxn()
				if err != nil {
					continue
				}
				heap.Push(transaction.DefaultHeap(dropList), nextTxn)
			}

			if !dropped {
				continue
			}
		}

		txnsDropped = append(txnsDropped, txn)
		tp.deleteTransactionFromMap(txn)
		atomic.AddInt32(&tp.txnCount, -1)
		atomic.AddInt64(&tp.txnSize, -int64(txn.GetSize()))
		currentTxnCount--
		currentTxnSize -= int64(txn.GetSize())

		if !isTxPoolFull(currentTxnCount, currentTxnSize) {
			break
		}
	}

	tp.blockValidationState.Lock()
	err := tp.CleanBlockValidationState(txnsDropped)
	tp.blockValidationState.Unlock()
	if err != nil {
		log.Errorf("CleanBlockValidationState error: %v", err)
	}

	bytesDropped := int64(0)
	for _, txn := range txnsDropped {
		bytesDropped += int64(txn.GetSize())
	}
	log.Infof("DropTxns: dropped %v txns (%v bytes)", len(txnsDropped), bytesDropped)

	if len(txnsDropped) > 0 {
		tp.setLastDroppedTxn(txnsDropped[len(txnsDropped)-1])
	} else {
		tp.setLastDroppedTxn(nil)
	}

	return
}

func (tp *TxnPool) AppendTxnPool(txn *transaction.Transaction) error {
	lastDroppedTxn := tp.getLastDroppedTxn()
	if lastDroppedTxn != nil && transaction.DefaultCompare(txn, lastDroppedTxn) <= 0 {
		return ErrRejectLowPriority
	}

	sender, err := txn.GetProgramHashes()
	if err != nil {
		return err
	}

	list, err := tp.getOrNewList(sender[0])
	if err != nil {
		return err
	}

	if _, err := list.GetByNonce(txn.UnsignedTx.Nonce); err != nil && list.Full() {
		return errors.New("account txpool full, too many transaction in list")
	}

	// 2. verify txn
	if err := txvalidator.VerifyTransaction(txn, chain.DefaultLedger.Store.GetHeight()+1); err != nil {
		return err
	}

	// 3. verify txn with ledger
	if err := chain.VerifyTransactionWithLedger(txn, chain.DefaultLedger.Store.GetHeight()+1); err != nil {
		return err
	}

	// 4. process txn
	if err := tp.processTx(txn); err != nil {
		return err
	}

	return nil
}

func (tp *TxnPool) getOrNewList(owner common.Uint160) (*NonceSortedTxs, error) {
	// check if the owner exsits.
	if _, ok := tp.TxLists.Load(owner); !ok {
		tp.TxLists.LoadOrStore(owner, NewNonceSortedTxs(owner, int(config.Parameters.TxPoolPerAccountTxCap)))
	}

	// 2. check if the txn exsits.
	v, ok := tp.TxLists.Load(owner)
	if !ok {
		return nil, errors.New("get txn list error")
	}
	list, ok := v.(*NonceSortedTxs)
	if !ok {
		return nil, errors.New("convert to NonceSortedTxs error")
	}

	return list, nil
}

func (tp *TxnPool) processTx(txn *transaction.Transaction) error {
	hash := txn.Hash()
	sender, err := txn.GetProgramHashes()
	if err != nil {
		return err
	}

	list, err := tp.getOrNewList(sender[0])
	if err != nil {
		return err
	}

	if list.ExistTx(hash) {
		return ErrDuplicatedTx
	}

	payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return err
	}

	tp.blockValidationState.Lock()
	defer tp.blockValidationState.Unlock()

	switch txn.UnsignedTx.Payload.Type {
	case pb.PayloadType_COINBASE_TYPE:
		return fmt.Errorf("Invalid txn type %v", txn.UnsignedTx.Payload.Type)
	case pb.PayloadType_SIG_CHAIN_TXN_TYPE:
		// sigchain txn should not be added to txn pool
		return nil
	case pb.PayloadType_NANO_PAY_TYPE:
		npPayload := payload.(*pb.NanoPay)
		key := nanoPayKey(string(npPayload.Sender), string(npPayload.Recipient), npPayload.Id)
		if v, ok := tp.NanoPayTxs.Load(key); ok {
			oldTxn := v.(*transaction.Transaction)
			oldPayload, err := transaction.Unpack(oldTxn.UnsignedTx.Payload)
			if err != nil {
				return err
			}
			oldNpPayload := oldPayload.(*pb.NanoPay)
			if npPayload.Amount < oldNpPayload.Amount || npPayload.TxnExpiration < oldNpPayload.TxnExpiration || npPayload.NanoPayExpiration < oldNpPayload.NanoPayExpiration {
				return ErrNanoPayReplace
			}
			err = tp.preReplaceTxn(oldTxn, txn)
			if err != nil {
				return err
			}
			tp.NanoPayTxs.Store(key, txn)
		} else {
			if err := tp.blockValidationState.VerifyTransactionWithBlock(txn, chain.DefaultLedger.Store.GetHeight()+1); err != nil {
				tp.blockValidationState.Reset()
				return err
			}
			tp.NanoPayTxs.Store(key, txn)
		}
	default:
		if oldTxn, err := list.GetByNonce(txn.UnsignedTx.Nonce); err == nil {
			err = tp.preReplaceTxn(oldTxn, txn)
			if err != nil {
				return err
			}

			if err := list.Replace(txn); err != nil {
				return err
			}
		} else if list.Full() {
			return errors.New("txpool per account list is full")
		} else {
			var expectNonce uint64
			if preNonce, err := list.GetLatestNonce(); err != nil {
				if err != ErrNonceSortedTxsEmpty {
					return errors.New("can not get nonce from txlist")
				}

				expectNonce = chain.DefaultLedger.Store.GetNonce(sender[0])
			} else {
				expectNonce = preNonce + 1
			}

			if txn.UnsignedTx.Nonce != expectNonce {
				return errors.New("nonce is not continuous")
			}

			if err := tp.blockValidationState.VerifyTransactionWithBlock(txn, chain.DefaultLedger.Store.GetHeight()+1); err != nil {
				tp.blockValidationState.Reset()
				return err
			}
			if err := list.Push(txn); err != nil {
				return err
			}
		}
	}

	tp.blockValidationState.Commit()

	tp.addTransactionToMap(txn)
	atomic.AddInt32(&tp.txnCount, 1)
	atomic.AddInt64(&tp.txnSize, int64(txn.GetSize()))

	return nil
}

func (tp *TxnPool) preReplaceTxn(oldTxn, newTxn *transaction.Transaction) error {
	log.Debug("replace old tx")

	if err := tp.CleanBlockValidationState([]*transaction.Transaction{oldTxn}); err != nil {
		return err
	}

	if err := tp.blockValidationState.VerifyTransactionWithBlock(newTxn, chain.DefaultLedger.Store.GetHeight()+1); err != nil {
		tp.blockValidationState.Reset()
		return err
	}

	tp.deleteTransactionFromMap(oldTxn)
	atomic.AddInt32(&tp.txnCount, -1)
	atomic.AddInt64(&tp.txnSize, -int64(oldTxn.GetSize()))

	return nil
}

func (tp *TxnPool) GetAddressList() map[common.Uint160]int {
	programHashes := make(map[common.Uint160]int)
	tp.TxLists.Range(func(k, v interface{}) bool {
		if programHash, ok := k.(common.Uint160); ok {
			if list, ok := v.(*NonceSortedTxs); ok {
				if listSize := list.Len(); listSize != 0 {
					programHashes[programHash] = listSize
				}
			}
		}

		return true
	})

	return programHashes
}

func (tp *TxnPool) GetAllTransactionsBySender(programHash common.Uint160) []*transaction.Transaction {
	txns := make([]*transaction.Transaction, 0)
	if v, ok := tp.TxLists.Load(programHash); ok {
		if list, ok := v.(*NonceSortedTxs); ok {
			for _, txn := range list.txs {
				txns = append(txns, txn)
			}
		}
	}
	tp.NanoPayTxs.Range(func(_, v interface{}) bool {
		txn := v.(*transaction.Transaction)
		payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
		if err != nil {
			log.Error(err)
			return true
		}
		pld := payload.(*pb.NanoPay)
		sender := common.BytesToUint160(pld.Sender)
		if sender == programHash {
			txns = append(txns, txn)
		}
		return true
	})

	sort.Sort(transaction.SortTxnsByNonce(txns))
	return txns
}

func (tp *TxnPool) GetTransaction(hash common.Uint256) *transaction.Transaction {
	if v, ok := tp.TxMap.Load(hash); ok {
		if txn, ok := v.(*transaction.Transaction); ok && txn != nil {
			return txn
		}
	}

	var found *transaction.Transaction
	tp.TxLists.Range(func(_, v interface{}) bool {
		if list, ok := v.(*NonceSortedTxs); ok {
			if list.ExistTx(hash) {
				found = list.txs[hash]
				return false
			}
		}
		return true
	})
	return found
}

func (tp *TxnPool) GetTxnByHash(hash common.Uint256) *transaction.Transaction {
	if v, ok := tp.TxMap.Load(hash); ok {
		if txn, ok := v.(*transaction.Transaction); ok && txn != nil {
			return txn
		}
	}
	return nil
}

func (tp *TxnPool) GetTxnByShortHash(shortHash []byte) *transaction.Transaction {
	if v, ok := tp.TxShortHashMap.Load(shortHashToKey(shortHash)); ok {
		if txn, ok := v.(*transaction.Transaction); ok && txn != nil {
			return txn
		}
	}
	return nil
}

func (tp *TxnPool) getTxsFromPool() []*transaction.Transaction {
	txs := make([]*transaction.Transaction, 0)
	tp.TxLists.Range(func(_, v interface{}) bool {
		if list, ok := v.(*NonceSortedTxs); ok {
			if tx, err := list.Seek(); err == nil {
				txs = append(txs, tx)
			}
		}
		return true
	})
	tp.NanoPayTxs.Range(func(_, v interface{}) bool {
		txs = append(txs, v.(*transaction.Transaction))
		return true
	})

	return txs
}

func (tp *TxnPool) GetAllTransactions() []*transaction.Transaction {
	txs := make([]*transaction.Transaction, 0)

	tp.TxLists.Range(func(k, v interface{}) bool {
		if list, ok := v.(*NonceSortedTxs); ok {
			if _, ok := k.(common.Uint160); ok {
				txs = append(txs, list.GetAllTransactions()...)
			}
		}
		return true
	})

	tp.NanoPayTxs.Range(func(_, v interface{}) bool {
		txs = append(txs, v.(*transaction.Transaction))
		return true
	})

	return txs
}

func (tp *TxnPool) GetSubscribers(topic string) []string {
	tp.blockValidationState.RLock()
	defer tp.blockValidationState.RUnlock()
	return tp.blockValidationState.GetSubscribers(topic)
}

func (tp *TxnPool) GetSubscribersWithMeta(topic string) map[string]string {
	tp.blockValidationState.RLock()
	defer tp.blockValidationState.RUnlock()
	return tp.blockValidationState.GetSubscribersWithMeta(topic)
}

func (tp *TxnPool) GetAllTransactionLists() map[common.Uint160][]*transaction.Transaction {
	txs := make(map[common.Uint160][]*transaction.Transaction)

	tp.TxLists.Range(func(k, v interface{}) bool {
		if list, ok := v.(*NonceSortedTxs); ok {
			if addr, ok := k.(common.Uint160); ok {
				txnList := list.GetAllTransactions()
				if len(txnList) != 0 {
					txs[addr] = txnList
				}
			}
		}

		return true
	})

	tp.NanoPayTxs.Range(func(_, v interface{}) bool {
		tx := v.(*transaction.Transaction)
		addr, _ := common.ToCodeHash(tx.Programs[0].Code)
		if _, ok := txs[addr]; !ok {
			txs[addr] = []*transaction.Transaction{tx}
		} else {
			txs[addr] = append(txs[addr], tx)
		}
		return true
	})

	return txs
}

func (tp *TxnPool) removeTransactions(txns []*transaction.Transaction) []*transaction.Transaction {
	txnsRemoved := make([]*transaction.Transaction, 0)

	// clean submitted txs
	for _, txn := range txns {
		txnsToRemove := make([]*transaction.Transaction, 0)

		switch txn.UnsignedTx.Payload.Type {
		case pb.PayloadType_COINBASE_TYPE:
			continue
		case pb.PayloadType_SIG_CHAIN_TXN_TYPE:
			continue
		case pb.PayloadType_NANO_PAY_TYPE:
			payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
			if err != nil {
				continue
			}
			npPayload := payload.(*pb.NanoPay)
			key := nanoPayKey(string(npPayload.Sender), string(npPayload.Recipient), npPayload.Id)
			if _, ok := tp.NanoPayTxs.Load(key); ok {
				tp.NanoPayTxs.Delete(key)
				txnsToRemove = append(txnsToRemove, txn)
			}
		default:
			sender, _ := common.ToCodeHash(txn.Programs[0].Code)
			txNonce := txn.UnsignedTx.Nonce

			if v, ok := tp.TxLists.Load(sender); ok {
				if list, ok := v.(*NonceSortedTxs); ok {
					if _, err := list.GetByNonce(txNonce); err == nil {
						nonce := list.getNonce(list.idx[0])
						txLst, err := list.PopN(uint16(txNonce - nonce + 1))
						if err == nil {
							txnsToRemove = append(txnsToRemove, txLst...)
						}
					}
				}
			}
		}

		for _, t := range txnsToRemove {
			txnsRemoved = append(txnsRemoved, t)
			tp.deleteTransactionFromMap(t)
			atomic.AddInt32(&tp.txnCount, -1)
			atomic.AddInt64(&tp.txnSize, -int64(t.GetSize()))
		}
	}

	tp.TxLists.Range(func(k, v interface{}) bool {
		if v.(*NonceSortedTxs).CleanIfEmpty() {
			tp.TxLists.Delete(k)
		}
		return true
	})

	return txnsRemoved
}

func (tp *TxnPool) CleanSubmittedTransactions(txns []*transaction.Transaction) error {
	txnsRemoved := tp.removeTransactions(txns)
	tp.blockValidationState.Lock()
	defer tp.blockValidationState.Unlock()
	return tp.CleanBlockValidationState(txnsRemoved)
}

func (tp *TxnPool) addTransactionToMap(txn *transaction.Transaction) {
	tp.TxMap.Store(txn.Hash(), txn)
	tp.TxShortHashMap.Store(shortHashToKey(txn.ShortHash(config.ShortHashSalt, config.ShortHashSize)), txn)
}

func (tp *TxnPool) deleteTransactionFromMap(txn *transaction.Transaction) {
	tp.TxMap.Delete(txn.Hash())
	tp.TxShortHashMap.Delete(shortHashToKey(txn.ShortHash(config.ShortHashSalt, config.ShortHashSize)))
}

func (tp *TxnPool) CleanBlockValidationState(txns []*transaction.Transaction) error {
	if err := tp.blockValidationState.CleanSubmittedTransactions(txns); err != nil {
		log.Errorf("[CleanBlockValidationState] couldn't clean txn from block validation state: %v", err)
		errMap := tp.blockValidationState.RefreshBlockValidationState(tp.GetAllTransactions())
		if len(errMap) > 0 {
			for txnHash, err := range errMap {
				log.Errorf("RefreshBlockValidationState error for txn %x: %v", txnHash, err)
			}
			invalidTxns := make([]*transaction.Transaction, 0, len(errMap))
			for _, txn := range txns {
				if _, ok := errMap[txn.Hash()]; ok {
					invalidTxns = append(invalidTxns, txn)
				}
			}
			tp.removeTransactions(invalidTxns)
		}
	}

	return nil
}

func (tp *TxnPool) GetTxnByCount(num int) (map[common.Uint256]*transaction.Transaction, error) {
	txmap := make(map[common.Uint256]*transaction.Transaction)

	txs := tp.getTxsFromPool()
	for _, tx := range txs {
		txmap[tx.Hash()] = tx
	}

	return txmap, nil
}

func (tp *TxnPool) GetNonceByTxnPool(addr common.Uint160) (uint64, error) {
	v, ok := tp.TxLists.Load(addr)
	if !ok {
		return 0, errors.New("no transactions in transaction pool")
	}
	list, ok := v.(*NonceSortedTxs)
	if !ok {
		return 0, errors.New("convert to NonceSortedTxs error")
	}

	pendingNonce, err := list.GetLatestNonce()
	if err != nil {
		return 0, err
	}
	expectedNonce := pendingNonce + 1

	return expectedNonce, nil
}

func shortHashToKey(shortHash []byte) string {
	return string(shortHash)
}
