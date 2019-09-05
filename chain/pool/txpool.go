package pool

import (
	"container/heap"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

func compareTxnPriority(txn1, txn2 *transaction.Transaction) int {
	if txn1.UnsignedTx.Fee > txn2.UnsignedTx.Fee {
		return 1
	}
	if txn1.UnsignedTx.Fee < txn2.UnsignedTx.Fee {
		return -1
	}
	if txn1.GetSize() > txn2.GetSize() {
		return -1
	}
	if txn1.GetSize() < txn2.GetSize() {
		return 1
	}
	return 0
}

type dropTxnsHeap []*transaction.Transaction

func (s dropTxnsHeap) Len() int            { return len(s) }
func (s dropTxnsHeap) Swap(i, j int)       { s[i], s[j] = s[j], s[i] }
func (s dropTxnsHeap) Less(i, j int) bool  { return compareTxnPriority(s[i], s[j]) < 0 }
func (s *dropTxnsHeap) Push(x interface{}) { *s = append(*s, x.(*transaction.Transaction)) }
func (s *dropTxnsHeap) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

var (
	ErrDuplicatedTx      = errors.New("duplicate transaction check failed")
	ErrRejectLowPriority = errors.New("txpool full, rejecting transaction with low priority")
)

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
				nonce, err := list.GetLatestNonce()
				if err == nil {
					txn, err := list.Get(nonce)
					if err == nil {
						dropList = append(dropList, txn)
					}
				}
			}
		}

		return true
	})

	tp.NanoPayTxs.Range(func(_, v interface{}) bool {
		dropList = append(dropList, v.(*transaction.Transaction))
		return true
	})

	heap.Init((*dropTxnsHeap)(&dropList))

	for {
		if len(dropList) == 0 {
			break
		}

		txn := heap.Pop((*dropTxnsHeap)(&dropList)).(*transaction.Transaction)

		switch txn.UnsignedTx.Payload.Type {
		case pb.NANO_PAY_TYPE:
			if _, ok := tp.NanoPayTxs.Load(txn.Hash()); !ok {
				continue
			}
			tp.NanoPayTxs.Delete(txn.Hash())
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
				nonce, err := list.GetLatestNonce()
				if err != nil {
					continue
				}
				nextTxn, err := list.Get(nonce)
				if err != nil {
					continue
				}
				heap.Push((*dropTxnsHeap)(&dropList), nextTxn)
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
	tp.CleanBlockValidationState(txnsDropped)
	tp.blockValidationState.Unlock()

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
	if lastDroppedTxn != nil && compareTxnPriority(txn, lastDroppedTxn) <= 0 {
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

	if _, err := list.Get(txn.UnsignedTx.Nonce); err != nil && list.Full() {
		return errors.New("account txpool full, too many transaction in list")
	}

	// 2. verify txn
	if err := chain.VerifyTransaction(txn, chain.DefaultLedger.Store.GetHeight()+1); err != nil {
		return err
	}

	// 3. verify txn with ledger
	if err := chain.VerifyTransactionWithLedger(txn); err != nil {
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

	_, err = transaction.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return err
	}

	switch txn.UnsignedTx.Payload.Type {
	case pb.COINBASE_TYPE:
		return fmt.Errorf("Invalid txn type %v", txn.UnsignedTx.Payload.Type)
	case pb.SIG_CHAIN_TXN_TYPE:
		// sigchain txn should not be added to txn pool
		return nil
	case pb.NANO_PAY_TYPE:
		tp.blockValidationState.Lock()
		defer tp.blockValidationState.Unlock()
		if err := tp.blockValidationState.VerifyTransactionWithBlock(txn, 0); err != nil {
			tp.blockValidationState.Reset()
			return err
		}
		tp.NanoPayTxs.Store(txn.Hash(), txn)
		tp.blockValidationState.Commit()
	default:
		if oldTxn, err := list.Get(txn.UnsignedTx.Nonce); err == nil {
			log.Debug("replace old tx")
			tp.blockValidationState.Lock()
			defer tp.blockValidationState.Unlock()
			if err := tp.CleanBlockValidationState([]*transaction.Transaction{oldTxn}); err != nil {
				return err
			}
			if err := tp.blockValidationState.VerifyTransactionWithBlock(txn, 0); err != nil {
				tp.blockValidationState.Reset()
				return err
			}

			if err := list.Add(txn); err != nil {
				return err
			}

			tp.deleteTransactionFromMap(oldTxn)
			atomic.AddInt32(&tp.txnCount, -1)
			atomic.AddInt64(&tp.txnSize, -int64(oldTxn.GetSize()))
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

			tp.blockValidationState.Lock()
			defer tp.blockValidationState.Unlock()
			if err := tp.blockValidationState.VerifyTransactionWithBlock(txn, 0); err != nil {
				tp.blockValidationState.Reset()
				return err
			}
			if err := list.Push(txn); err != nil {
				return err
			}
		}

		tp.blockValidationState.Commit()
	}

	tp.addTransactionToMap(txn)
	atomic.AddInt32(&tp.txnCount, 1)
	atomic.AddInt64(&tp.txnSize, int64(txn.GetSize()))

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
	tp.NanoPayTxs.Range(func(k, v interface{}) bool {
		txns = append(txns, v.(*transaction.Transaction))
		return true
	})

	sort.Sort(sortTxnsByNonce(txns))
	return txns
}

func (tp *TxnPool) GetTransaction(hash common.Uint256) *transaction.Transaction {
	if v, ok := tp.TxMap.Load(hash); ok {
		if txn, ok := v.(*transaction.Transaction); ok && txn != nil {
			return txn
		}
	}

	if np, ok := tp.NanoPayTxs.Load(hash); ok {
		return np.(*transaction.Transaction)
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
	tp.NanoPayTxs.Range(func(k, v interface{}) bool {
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

	tp.NanoPayTxs.Range(func(k, v interface{}) bool {
		txs = append(txs, v.(*transaction.Transaction))
		return true
	})

	return txs
}

func (tp *TxnPool) GetSubscribers(topic string) []string {
	return tp.blockValidationState.GetSubscribers(topic)
}

func (tp *TxnPool) GetSubscribersWithMeta(topic string) map[string]string {
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

	tp.NanoPayTxs.Range(func(k, v interface{}) bool {
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

func (tp *TxnPool) CleanSubmittedTransactions(txns []*transaction.Transaction) error {
	txnsRemoved := make([]*transaction.Transaction, 0)

	// clean submitted txs
	for _, txn := range txns {
		txnsToRemove := make([]*transaction.Transaction, 0)

		switch txn.UnsignedTx.Payload.Type {
		case pb.COINBASE_TYPE:
			continue
		case pb.SIG_CHAIN_TXN_TYPE:
			continue
		case pb.NANO_PAY_TYPE:
			tp.NanoPayTxs.Delete(txn.Hash())
			txnsToRemove = append(txnsToRemove, txn)
		default:
			sender, _ := common.ToCodeHash(txn.Programs[0].Code)
			txNonce := txn.UnsignedTx.Nonce

			if v, ok := tp.TxLists.Load(sender); ok {
				if list, ok := v.(*NonceSortedTxs); ok {
					if _, err := list.Get(txNonce); err == nil {
						nonce := list.getNonce(list.idx[0])
						for i := 0; uint64(i) <= txNonce-nonce; i++ {
							t, err := list.Pop()
							if err == nil {
								txnsToRemove = append(txnsToRemove, t)
							}
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
		listLen := v.(*NonceSortedTxs).Len()
		if listLen == 0 {
			v.(*NonceSortedTxs).txs = nil
			tp.TxLists.Delete(k)
		}
		return true
	})

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
		return tp.blockValidationState.RefreshBlockValidationState(tp.GetAllTransactions())
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
