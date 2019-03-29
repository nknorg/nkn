package pool

import (
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
	ErrDuplicateName         = errors.New("Duplicate NameService operation in one block")
	ErrNoNameRegistered      = errors.New("name already has not be registered")
	ErrDuplicateSubscription = errors.New("Duplicate subscription in one block")
	ErrSubscriptionLimit     = errors.New("Subscription limit exceeded in one block")
	ErrNonOptimalSigChain    = errors.New("This SigChain is NOT optimal choice")
)

// TxnPool is a list of txns that need to by add to ledger sent by user.
type TxnPool struct {
	mu          sync.RWMutex
	TxLists     map[common.Uint160]*NonceSortedTxs // NonceSortedTxs instance to store user's account.
	SigChainTxs map[common.Uint256]*Transaction    // tx with sigchain type.

}

func NewTxPool() *TxnPool {
	return &TxnPool{
		TxLists:     make(map[common.Uint160]*NonceSortedTxs),
		SigChainTxs: make(map[common.Uint256]*Transaction),
	}
}

func (tp *TxnPool) AppendTxnPool(txn *Transaction) error {
	//1. process all Orphens
	for _, list := range tp.TxLists {
		list.ProcessOrphans(tp.processTx)
	}
	for _, list := range tp.TxLists {
		list.CleanOrphans(nil)
	}

	// 2. verify txn with ledger
	if err := tp.verifyTransactionWithLedger(txn); err != nil {
		return err
	}

	// 3. process txn
	if err := tp.processTx(txn); err != nil {
		return err
	}

	// 3. process orphans
	for _, list := range tp.TxLists {
		list.ProcessOrphans(tp.processTx)
	}
	for _, list := range tp.TxLists {
		list.CleanOrphans(nil)
	}

	tp.Dump()

	return nil
}

func (tp *TxnPool) processTx(txn *Transaction) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if txn.UnsignedTx.Payload.Type == pb.CommitType {
		tp.SigChainTxs[txn.Hash()] = txn
		return nil
	}

	hash := txn.Hash()
	sender, _ := common.ToCodeHash(txn.Programs[0].Code)

	//1. check if the sender is exsit.
	if _, ok := tp.TxLists[sender]; !ok {
		tp.TxLists[sender] = NewNonceSortedTxs(sender, DefaultCap)
	}

	//2. check if the txn is exsit.
	list := tp.TxLists[sender]
	if list.ExistTx(hash) {
		return ErrDuplicatedTx
	}

	//check balance
	if txn.UnsignedTx.Payload.Type == pb.TransferAssetType {
		pl, err := Unpack(txn.UnsignedTx.Payload)
		if err != nil {
			return err
		}
		ta := pl.(*pb.TransferAsset)

		amount := chain.DefaultLedger.Store.GetBalance(sender)
		allInList := list.Totality() + common.Fixed64(ta.Amount)

		if amount < allInList {
			return errors.New("not sufficient funds")
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
	for _, list := range tp.TxLists {
		for hash, txn := range list.txs {
			txns[hash] = txn
		}
		for hash, txn := range list.orphans {
			txns[hash] = txn
		}
	}

	return txns
}

func (tp *TxnPool) GetTransaction(hash common.Uint256) *Transaction {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	for _, list := range tp.TxLists {
		if list.ExistTx(hash) {
			return list.txs[hash]
		}
	}

	return nil
}

func (tp *TxnPool) getTxsFromPool() []*Transaction {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
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
		if txn.UnsignedTx.Payload.Type == pb.CoinbaseType ||
			txn.UnsignedTx.Payload.Type == pb.CommitType {
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

				// clean invalid txs
				list.CleanOrphans([]*Transaction{txn})
			}
		}

	}

	// clean sigchaintxs
	tp.SigChainTxs = make(map[common.Uint256]*Transaction)

	return nil
}

func (tp *TxnPool) GetTxnByCount(num int) (map[common.Uint256]*Transaction, error) {
	txmap := make(map[common.Uint256]*Transaction)

	txs := tp.getTxsFromPool()
	for _, tx := range txs {
		txmap[tx.Hash()] = tx
	}

	tp.Dump()
	return txmap, nil
}

func (tp *TxnPool) Dump() {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	for _, list := range tp.TxLists {
		list.Dump()
	}

	log.Error("SigChainTxs:")
	for h, _ := range tp.SigChainTxs {
		log.Error(h.ToHexString())
	}
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

	// get signature chain from commit transaction then add it to POR server
	if txn.UnsignedTx.Payload.Type == pb.CommitType {
		added, err := por.GetPorServer().AddSigChainFromTx(txn, chain.DefaultLedger.Store.GetHeight())
		if err != nil {
			return err
		}
		if !added {
			return ErrDuplicatedTx
		}
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
			return ErrDuplicateName
		}
		if err != leveldb.ErrNotFound {
			return err
		}

		registrant, err := chain.DefaultLedger.Store.GetRegistrant(pld.Name)
		if registrant != nil {
			return ErrNoNameRegistered
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
