package blockchain

import (
	"math"

	. "github.com/nknorg/nkn/common"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/signature"
	"github.com/nknorg/nkn/types"
	"github.com/nknorg/nkn/util/log"
)

const (
	SubscriptionsLimit      = 1000
	MaxSubscriptionDuration = 65535
)

var Store TxnStore

type TxnStore interface {
	GetTransaction(hash Uint256) (*types.Transaction, error)
	IsDoubleSpend(tx *types.Transaction) bool
	IsTxHashDuplicate(txhash Uint256) bool
	GetName(registrant []byte) (*string, error)
	GetRegistrant(name string) ([]byte, error)
	IsSubscribed(subscriber []byte, identifier string, topic string) (bool, error)
	GetSubscribersCount(topic string) int
}

type Iterator interface {
	Iterate(handler func(item *types.Transaction) ErrCode) ErrCode
}

// VerifyTransaction verifys received single transaction
func VerifyTransaction(Tx *types.Transaction) ErrCode {
	if err := CheckAssetPrecision(Tx); err != nil {
		log.Warning("[VerifyTransaction],", err)
		return ErrAssetPrecision
	}

	if err := CheckTransactionBalance(Tx); err != nil {
		log.Warning("[VerifyTransaction],", err)
		return ErrTransactionBalance
	}

	if err := CheckAttributeProgram(Tx); err != nil {
		log.Warning("[VerifyTransaction],", err)
		return ErrAttributeProgram
	}

	if err := CheckTransactionContracts(Tx); err != nil {
		log.Warning("[VerifyTransaction],", err)
		return ErrTransactionContracts
	}

	if err := CheckTransactionPayload(Tx); err != nil {
		log.Warning("[VerifyTransaction],", err)
		return ErrTransactionPayload
	}

	return ErrNoError
}

// VerifyTransactionWithBlock verifys a transaction with current transaction pool in memory
func VerifyTransactionWithBlock(iterator Iterator) ErrCode {
	//initial
	//txnlist := make(map[Uint256]struct{}, 0)
	//registeredNames := make(map[string]struct{}, 0)
	//nameRegistrants := make(map[string]struct{}, 0)

	//type subscription struct{ topic, subscriber string }
	//subscriptions := make(map[subscription]struct{}, 0)

	//subscriptionCount := make(map[string]int, 0)

	////start check
	//return iterator.Iterate(func(txn *Transaction) ErrCode {
	//	//1.check weather have duplicate transaction.
	//	if _, exist := txnlist[txn.Hash()]; exist {
	//		log.Warning("[VerifyTransactionWithBlock], duplicate transaction exist in block.")
	//		return ErrDuplicatedTx
	//	} else {
	//		txnlist[txn.Hash()] = struct{}{}
	//	}
	//	//3.check issue amount
	//	switch txn.TxType {
	//	case Coinbase:
	//		coinbase := txn.Payload.(*payload.Coinbase)
	//		if coinbase.Amount != Fixed64(config.DefaultMiningReward*StorageFactor) {
	//			log.Warning("Mining reward incorrectly.")
	//			return ErrMineReward
	//		}

	//	case RegisterName:
	//		namePayload := txn.Payload.(*payload.RegisterName)

	//		name := namePayload.Name
	//		if _, ok := registeredNames[name]; ok {
	//			log.Warning("[VerifyTransactionWithBlock], duplicate name exist in block.")
	//			return ErrDuplicateName
	//		}
	//		registeredNames[name] = struct{}{}

	//		registrant := BytesToHexString(namePayload.Registrant)
	//		if _, ok := nameRegistrants[registrant]; ok {
	//			log.Warning("[VerifyTransactionWithBlock], duplicate registrant exist in block.")
	//			return ErrDuplicateName
	//		}
	//		nameRegistrants[registrant] = struct{}{}
	//	case DeleteName:
	//		namePayload := txn.Payload.(*payload.DeleteName)

	//		registrant := BytesToHexString(namePayload.Registrant)
	//		if _, ok := nameRegistrants[registrant]; ok {
	//			log.Warning("[VerifyTransactionWithBlock], duplicate registrant exist in block.")
	//			return ErrDuplicateName
	//		}
	//		nameRegistrants[registrant] = struct{}{}
	//	case Subscribe:
	//		subscribePayload := txn.Payload.(*payload.Subscribe)
	//		topic := subscribePayload.Topic
	//		key := subscription{topic, subscribePayload.SubscriberString()}
	//		if _, ok := subscriptions[key]; ok {
	//			log.Warning("[VerifyTransactionWithBlock], duplicate subscription exist in block.")
	//			return ErrDuplicateSubscription
	//		}
	//		subscriptions[key] = struct{}{}

	//		if _, ok := subscriptionCount[topic]; !ok {
	//			subscriptionCount[topic] = Store.GetSubscribersCount(topic)
	//		}
	//		if subscriptionCount[topic] >= SubscriptionsLimit {
	//			log.Warning("[VerifyTransactionWithBlock], subscription limit exceeded in block.")
	//			return ErrSubscriptionLimit
	//		}
	//		subscriptionCount[topic]++
	//	}

	return ErrNoError
	//})
}

// VerifyTransactionWithLedger verifys a transaction with history transaction in ledger
func VerifyTransactionWithLedger(Tx *types.Transaction) ErrCode {
	if IsDoubleSpend(Tx) {
		log.Info("[VerifyTransactionWithLedger] IsDoubleSpend check faild.")
		return ErrDoubleSpend
	}
	if exist := Store.IsTxHashDuplicate(Tx.Hash()); exist {
		log.Info("[VerifyTransactionWithLedger] duplicate transaction check faild.")
		return ErrTxHashDuplicate
	}
	return ErrNoError
}

func IsDoubleSpend(tx *types.Transaction) bool {
	return Store.IsDoubleSpend(tx)
}

func CheckAssetPrecision(Tx *types.Transaction) error {
	return nil
}

func CheckTransactionBalance(txn *types.Transaction) error {
	//if txn.TxType == Coinbase {
	//	return nil
	//}

	return nil
}

func CheckAttributeProgram(Tx *types.Transaction) error {
	//TODO: implement CheckAttributeProgram
	return nil
}

func CheckTransactionContracts(Tx *types.Transaction) error {
	flag, err := signature.VerifySignableData(Tx)
	if flag && err == nil {
		return nil
	} else {
		return err
	}
}

func checkAmountPrecise(amount Fixed64, precision byte) bool {
	return amount.GetData()%int64(math.Pow(10, 8-float64(precision))) != 0
}

func CheckTransactionPayload(txn *types.Transaction) error {

	//switch pld := txn.Payload.(type) {
	//case *payload.TransferAsset:
	//case *payload.Coinbase:
	//case *payload.Commit:
	//case *payload.RegisterName:
	//	match, err := regexp.MatchString("([a-z]{8,12})", pld.Name)
	//	if err != nil {
	//		return err
	//	}
	//	if !match {
	//		return errors.New(fmt.Sprintf("name %s should only contain a-z and have length 8-12", pld.Name))
	//	}

	//	name, err := Store.GetName(pld.Registrant)
	//	if name != nil {
	//		return errors.New(fmt.Sprintf("pubKey %+v already has registered name %s", pld.Registrant, *name))
	//	}
	//	if err != leveldb.ErrNotFound {
	//		return err
	//	}

	//	registrant, err := Store.GetRegistrant(pld.Name)
	//	if registrant != nil {
	//		return errors.New(fmt.Sprintf("name %s is already registered for pubKey %+v", pld.Name, registrant))
	//	}
	//	if err != leveldb.ErrNotFound {
	//		return err
	//	}
	//case *payload.DeleteName:
	//	name, err := Store.GetName(pld.Registrant)
	//	if err != leveldb.ErrNotFound {
	//		return err
	//	}
	//	if name == nil {
	//		return errors.New(fmt.Sprintf("no name registered for pubKey %+v", pld.Registrant))
	//	}
	//case *payload.Subscribe:
	//	duration := pld.Duration
	//	if duration > MaxSubscriptionDuration {
	//		return errors.New(fmt.Sprintf("subscription duration %d can't be bigger than %d", duration, MaxSubscriptionDuration))
	//	}

	//	topic := pld.Topic
	//	match, err := regexp.MatchString("(^[a-z][a-z0-9-_.~+%]{2,254}$)", topic)
	//	if err != nil {
	//		return err
	//	}
	//	if !match {
	//		return errors.New(fmt.Sprintf("topic %s should only contain a-z and have length 8-12", topic))
	//	}

	//	subscribed, err := Store.IsSubscribed(pld.Subscriber, pld.Identifier, topic)
	//	if err != nil {
	//		return err
	//	}
	//	if subscribed {
	//		return errors.New(fmt.Sprintf("subscriber %s already subscribed to %s", pld.SubscriberString(), topic))
	//	}

	//	subscriptionCount := Store.GetSubscribersCount(topic)
	//	if subscriptionCount >= SubscriptionsLimit {
	//		return errors.New(fmt.Sprintf("subscribtion count to %s can't be more than %d", topic, subscriptionCount))
	//	}
	//default:
	//	return errors.New("[txValidator],invalidate transaction payload type.")
	//}
	return nil
}
