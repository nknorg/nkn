package pool

import (
	"errors"
	"sort"
	"sync"

	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

var (
	ErrDuplicatedTx = errors.New("duplicate transaction check failed")
)

// TxnPool is a list of txns that need to by add to ledger sent by user.
type TxnPool struct {
	TxLists              sync.Map // NonceSortedTxs instance to store user's account.
	TxMap                sync.Map
	TxShortHashMap       sync.Map
	NanoPayTxs           sync.Map // tx with nano pay type.
	blockValidationState *chain.BlockValidationState
}

func NewTxPool() *TxnPool {
	return &TxnPool{blockValidationState: chain.NewBlockValidationState()}
}

func (tp *TxnPool) AppendTxnPool(txn *transaction.Transaction) error {
	sender, err := txn.GetProgramHashes()
	if err != nil {
		return err
	}

	list, err := tp.getOrNewList(sender[0])
	if err != nil {
		return err
	}

	if _, err := list.Get(txn.UnsignedTx.Nonce); err != nil && list.Full() {
		return errors.New("txpool full, too many transaction in txpool")
	}

	// 2. verify txn
	if err := chain.VerifyTransaction(txn); err != nil {
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
		tp.TxLists.LoadOrStore(owner, NewNonceSortedTxs(owner, config.Parameters.TxPoolCap))
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
			log.Warning("replace old tx")
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
		} else if list.Full() {
			return errors.New("txpool is full")
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
				return errors.New("the nonce is not continuous")
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
	txnsInPool := make([]*transaction.Transaction, 0)

	// clean submitted txs
	for _, txn := range txns {
		switch txn.UnsignedTx.Payload.Type {
		case pb.COINBASE_TYPE:
			continue
		case pb.SIG_CHAIN_TXN_TYPE:
			continue
		case pb.NANO_PAY_TYPE:
			tp.NanoPayTxs.Delete(txn.Hash())
		default:
			sender, _ := common.ToCodeHash(txn.Programs[0].Code)
			txNonce := txn.UnsignedTx.Nonce

			if v, ok := tp.TxLists.Load(sender); ok {
				if list, ok := v.(*NonceSortedTxs); ok {
					if _, err := list.Get(txNonce); err == nil {
						nonce := list.getNonce(list.idx[0])
						for i := 0; uint64(i) <= txNonce-nonce; i++ {
							list.Pop()
						}
					}
				}
			}
		}

		if _, ok := tp.TxMap.Load(txn.Hash()); ok {
			txnsInPool = append(txnsInPool, txn)
		}
		tp.deleteTransactionFromMap(txn)
	}

	tp.blockValidationState.Lock()
	defer tp.blockValidationState.Unlock()
	return tp.CleanBlockValidationState(txnsInPool)
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
