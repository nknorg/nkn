package pool

import (
	"bytes"
	"errors"
	"sync"

	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	. "github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nnet/log"
	"github.com/syndtr/goleveldb/leveldb"
)

const (
	DefaultCap               = 1024
	ExclusivedSigchainHeight = 3
)

var (
	ErrDuplicatedTx          = errors.New("duplicate transaction check faild")
	ErrDoubleSpend           = errors.New("IsDoubleSpend check faild")
	ErrTxnType               = errors.New("invalidate transaction payload type")
	ErrNonceTooLow           = errors.New("nonce is too low")
	ErrDuplicateName         = errors.New("name has be registered already")
	ErrDuplicateRegistrant   = errors.New("Registrant has already registered this name")
	ErrNoNameRegistered      = errors.New("name has not be registered yet")
	ErrDuplicateSubscription = errors.New("Duplicate subscription in one block")
	ErrSubscriptionLimit     = errors.New("Subscription limit exceeded in one block")
	ErrNonOptimalSigChain    = errors.New("This SigChain is NOT optimal choice")
)

// TxnPool is a list of txns that need to by add to ledger sent by user.
type TxnPool struct {
	TxLists sync.Map // NonceSortedTxs instance to store user's account.
}

func NewTxPool() *TxnPool {
	return &TxnPool{}
}

func (tp *TxnPool) AppendTxnPool(txn *Transaction) error {
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

	// 2. verify txn with ledger
	if err := tp.verifyTransactionWithLedger(txn); err != nil {
		return err
	}

	// 3. process txn
	if err := tp.processTx(txn); err != nil {
		return err
	}

	// 3. process orphans
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

func (tp *TxnPool) processTx(txn *Transaction) error {
	hash := txn.Hash()
	sender, _ := common.ToCodeHash(txn.Programs[0].Code)

	// 1. check if the sender exsits.
	if _, ok := tp.TxLists.Load(sender); !ok {
		tp.TxLists.LoadOrStore(sender, NewNonceSortedTxs(sender, DefaultCap))
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

	pl, err := Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return err
	}
	switch txn.UnsignedTx.Payload.Type {
	case pb.CommitType:
		// sigchain txn should not be added to txn pool
		return nil
	case pb.TransferAssetType:
		ta := pl.(*pb.TransferAsset)
		amount := chain.DefaultLedger.Store.GetBalance(sender)
		allInList := list.Totality() + common.Fixed64(ta.Amount)

		if amount < allInList {
			return errors.New("not sufficient funds")
		}
	case pb.RegisterNameType:
		rn := pl.(*pb.RegisterName)
		for _, tx := range list.txs {
			if tx.UnsignedTx.Payload.Type != pb.RegisterNameType {
				continue
			}
			pld, err := Unpack(tx.UnsignedTx.Payload)
			if err != nil {
				return err
			}
			payloadRn := pld.(*pb.RegisterName)
			//TODO need compare nonce?
			if bytes.Equal(payloadRn.Registrant, rn.Registrant) {
				return ErrDuplicateRegistrant
			}

			if payloadRn.Name == rn.Name {
				log.Error(payloadRn.Name, rn.Name)
				return ErrDuplicateName
			}
		}
	case pb.DeleteNameType:
		rn := pl.(*pb.DeleteName)
		for _, tx := range list.txs {
			if tx.UnsignedTx.Payload.Type != pb.DeleteNameType {
				continue
			}
			pld, err := Unpack(tx.UnsignedTx.Payload)
			if err != nil {
				return err
			}
			payloadRn := pld.(*pb.DeleteName)
			if bytes.Equal(payloadRn.Registrant, rn.Registrant) {
				return ErrDuplicateRegistrant
			}

			if payloadRn.Name == rn.Name {
				log.Error(payloadRn.Name, rn.Name)
				return ErrDuplicateName
			}
		}
	case pb.SubscribeType:
		rn := pl.(*pb.Subscribe)
		for _, tx := range list.txs {
			if tx.UnsignedTx.Payload.Type != pb.SubscribeType {
				continue
			}
			pld, err := Unpack(tx.UnsignedTx.Payload)
			if err != nil {
				return err
			}
			payloadRn := pld.(*pb.Subscribe)
			if bytes.Equal(payloadRn.Subscriber, rn.Subscriber) &&
				payloadRn.Identifier == rn.Identifier &&
				payloadRn.Topic == rn.Topic &&
				payloadRn.Bucket == rn.Bucket {
				return ErrDuplicateSubscription
			}

		}
	}

	if !list.Empty() {
		//replace old tx that has same nonce.
		if _, err := list.Get(txn.UnsignedTx.Nonce); err == nil {
			log.Warning("replace old tx")
			//TODO need more fee
			return list.Add(txn)
		}

		if list.Full() {
			return errors.New("full")
		}

		preNonce, _ := list.GetLatestNonce()

		// check total account in it
		if txn.UnsignedTx.Nonce == preNonce+1 {
			return list.Push(txn)
		}
	} else {
		// compare with DB
		expectNonce := chain.DefaultLedger.Store.GetNonce(sender)
		if txn.UnsignedTx.Nonce == expectNonce {
			return list.Push(txn)
		}
	}

	// 3. add to orphans
	if list.GetOrphanTxn(hash) != nil {
		return ErrDuplicatedTx
	}

	list.AddOrphanTxn(txn)

	return nil
}

func (tp *TxnPool) GetAllTransactions() map[common.Uint256]*Transaction {
	txns := make(map[common.Uint256]*Transaction)
	tp.TxLists.Range(func(_, v interface{}) bool {
		if list, ok := v.(*NonceSortedTxs); ok {
			for hash, txn := range list.txs {
				txns[hash] = txn
			}
			for hash, txn := range list.orphans {
				txns[hash] = txn
			}
		}
		return true
	})

	return txns
}

func (tp *TxnPool) GetTransaction(hash common.Uint256) *Transaction {
	var found *Transaction
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

func (tp *TxnPool) getTxsFromPool() []*Transaction {
	txs := make([]*Transaction, 0)
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

func (tp *TxnPool) CleanSubmittedTransactions(txns []*Transaction) error {
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
					list.CleanOrphans([]*Transaction{txn})
				}
			}
		}
	}

	return nil
}

func (tp *TxnPool) GetTxnByCount(num int) (map[common.Uint256]*Transaction, error) {
	txmap := make(map[common.Uint256]*Transaction)

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

func (tp *TxnPool) verifyTransactionWithLedger(txn *Transaction) error {
	if err := chain.VerifyTransaction(txn); err != nil {
		return err
	}

	if chain.DefaultLedger.Store.IsDoubleSpend(txn) {
		return ErrDoubleSpend
	}

	if chain.DefaultLedger.Store.IsTxHashDuplicate(txn.Hash()) {
		return ErrDuplicatedTx
	}

	sender, _ := common.ToCodeHash(txn.Programs[0].Code)
	nonce := chain.DefaultLedger.Store.GetNonce(sender)

	payload, err := Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return err
	}

	switch txn.UnsignedTx.Payload.Type {
	case pb.CoinbaseType:
		return ErrTxnType
	case pb.CommitType:
		added, err := por.GetPorServer().AddSigChainFromTx(txn, chain.DefaultLedger.Store.GetHeight())
		if err != nil {
			return err
		}
		if !added {
			return ErrNonOptimalSigChain
		}
	case pb.TransferAssetType:
		if txn.UnsignedTx.Nonce < nonce {
			return ErrNonceTooLow
		}
	case pb.RegisterNameType:
		if txn.UnsignedTx.Nonce < nonce {
			return ErrNonceTooLow
		}

		pld := payload.(*pb.RegisterName)
		name, err := chain.DefaultLedger.Store.GetName(pld.Registrant)
		if name != nil {
			return ErrDuplicateRegistrant
		}
		if err != leveldb.ErrNotFound {
			return err
		}

		registrant, err := chain.DefaultLedger.Store.GetRegistrant(pld.Name)
		if registrant != nil {
			return ErrDuplicateRegistrant
		}
		if err != leveldb.ErrNotFound {
			return err
		}
	case pb.DeleteNameType:
		if txn.UnsignedTx.Nonce < nonce {
			return ErrNonceTooLow
		}

		pld := payload.(*pb.DeleteName)
		name, err := chain.DefaultLedger.Store.GetName(pld.Registrant)
		if err != leveldb.ErrNotFound {
			return err
		}
		if name == nil {
			return ErrNoNameRegistered
		}
	case pb.SubscribeType:
		if txn.UnsignedTx.Nonce < nonce {
			return ErrNonceTooLow
		}

		pld := payload.(*pb.Subscribe)
		subscribed, err := chain.DefaultLedger.Store.IsSubscribed(pld.Subscriber, pld.Identifier, pld.Topic, pld.Bucket)
		if err != nil {
			return err
		}
		if subscribed {
			return ErrDuplicateSubscription
		}

		subscriptionCount := chain.DefaultLedger.Store.GetSubscribersCount(pld.Topic, pld.Bucket)
		if subscriptionCount >= SubscriptionsLimit {
			return ErrSubscriptionLimit
		}

	default:
		return ErrTxnType
	}

	return nil
}
