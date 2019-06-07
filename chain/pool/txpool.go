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
	blockValidationState *chain.BlockValidationState
}

func NewTxPool() *TxnPool {
	return &TxnPool{blockValidationState: chain.NewBlockValidationState()}
}

func (tp *TxnPool) AppendTxnPool(txn *transaction.Transaction) error {
	// 1. process all Orphens
	tp.TxLists.Range(func(_, v interface{}) bool {
		if list, ok := v.(*NonceSortedTxs); ok {
			list.ProcessOrphans(tp.processTx)
		}
		return true
	})
	tp.TxLists.Range(func(_, v interface{}) bool {
		if list, ok := v.(*NonceSortedTxs); ok {
			list.CleanOrphans(nil)
		}
		return true
	})

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

	// 5. process orphans
	tp.TxLists.Range(func(_, v interface{}) bool {
		if list, ok := v.(*NonceSortedTxs); ok {
			list.ProcessOrphans(tp.processTx)
		}
		return true
	})
	tp.TxLists.Range(func(_, v interface{}) bool {
		if list, ok := v.(*NonceSortedTxs); ok {
			list.CleanOrphans(nil)
		}
		return true
	})

	return nil
}

func (tp *TxnPool) processTx(txn *transaction.Transaction) error {
	hash := txn.Hash()
	sender, _ := common.ToCodeHash(txn.Programs[0].Code)

	// 1. check if the sender exsits.
	if _, ok := tp.TxLists.Load(sender); !ok {
		tp.TxLists.LoadOrStore(sender, NewNonceSortedTxs(sender, config.DefaultTxPoolCap, config.DefaultTxPoolOrphanCap))
	}

	// 2. check if the txn exsits.
	v, ok := tp.TxLists.Load(sender)
	if !ok {
		return errors.New("get txn list error")
	}
	list, ok := v.(*NonceSortedTxs)
	if !ok {
		return errors.New("convert to NonceSortedTxs error")
	}
	if list.ExistTx(hash) {
		return ErrDuplicatedTx
	}

	_, err := transaction.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return err
	}

	if txn.UnsignedTx.Payload.Type == pb.CommitType {
		// sigchain txn should not be added to txn pool
		return nil
	}

	isOrphan := true
	replace := false
	if !list.Empty() {
		//replace old tx that has same nonce.
		if _, err := list.Get(txn.UnsignedTx.Nonce); err == nil {
			log.Warning("replace old tx")
			//TODO need more fee
			isOrphan = false
			replace = true
		} else if list.Full() {
			return errors.New("txpool is full")
		} else if preNonce, _ := list.GetLatestNonce(); txn.UnsignedTx.Nonce == preNonce+1 {
			isOrphan = false
		}
	} else {
		// compare with DB
		expectNonce := chain.DefaultLedger.Store.GetNonce(sender)
		if txn.UnsignedTx.Nonce == expectNonce {
			isOrphan = false
		}
	}

	if !isOrphan {
		if err := tp.blockValidationState.VerifyTransactionWithBlock(txn, nil); err != nil {
			return err
		}

		if replace {
			if err := list.Add(txn); err != nil {
				return err
			}
		} else {
			if err := list.Push(txn); err != nil {
				return err
			}
		}
	} else {
		if list.GetOrphanTxn(hash) != nil {
			return ErrDuplicatedTx
		}
		if err := list.AddOrphanTxn(txn); err != nil {
			return err
		}
	}

	tp.TxMap.Store(txn.Hash(), txn)
	tp.TxShortHashMap.Store(shortHashToKey(txn.ShortHash(config.ShortHashSalt, config.ShortHashSize)), txn)

	return nil
}

func (tp *TxnPool) GetAddressList() map[common.Uint160]int {
	programHashes := make(map[common.Uint160]int)
	tp.TxLists.Range(func(k, v interface{}) bool {
		if programHash, ok := k.(common.Uint160); ok {
			if list, ok := v.(*NonceSortedTxs); ok {
				count := len(list.txs) + len(list.orphans)
				programHashes[programHash] = count
			}
		}

		return true
	})

	return programHashes
}

func (tp *TxnPool) GetAllTransactions(programHash common.Uint160) []*transaction.Transaction {
	txns := make([]*transaction.Transaction, 0)
	if v, ok := tp.TxLists.Load(programHash); ok {
		if list, ok := v.(*NonceSortedTxs); ok {
			for _, txn := range list.txs {
				txns = append(txns, txn)
			}
			for _, txn := range list.orphans {
				txns = append(txns, txn)
			}
		}
	}

	sort.Sort(sortTxnsByNonce(txns))
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

	return txs
}

func (tp *TxnPool) CleanSubmittedTransactions(txns []*transaction.Transaction) error {
	// clean submitted txs
	for _, txn := range txns {
		if txn.UnsignedTx.Payload.Type == pb.CoinbaseType ||
			txn.UnsignedTx.Payload.Type == pb.CommitType {
			continue
		}

		sender, _ := common.ToCodeHash(txn.Programs[0].Code)
		txNonce := txn.UnsignedTx.Nonce

		if v, ok := tp.TxLists.Load(sender); ok {
			if list, ok := v.(*NonceSortedTxs); ok {
				if _, err := list.Get(txNonce); err == nil {
					nonce := list.getNonce(list.idx[0])
					for i := 0; uint64(i) <= txNonce-nonce; i++ {
						list.Pop()
					}

					// clean invalid txs
					list.CleanOrphans([]*transaction.Transaction{txn})
					list.ProcessOrphans(tp.processTx)
				}
			}
		}

		tp.TxMap.Delete(txn.Hash())
		tp.TxShortHashMap.Delete(shortHashToKey(txn.ShortHash(config.ShortHashSalt, config.ShortHashSize)))
	}

	return tp.blockValidationState.CleanSubmittedTransactions(txns)
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
