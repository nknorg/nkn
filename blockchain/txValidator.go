package blockchain

import (
	"errors"
	"fmt"
	"math"
	"regexp"

	. "github.com/nknorg/nkn/common"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/signature"
	"github.com/nknorg/nkn/types"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nnet/log"
	"github.com/syndtr/goleveldb/leveldb"
)

const (
	SubscriptionsLimit      = 1000
	MaxSubscriptionDuration = 65535
)

// VerifyTransaction verifys received single transaction
func VerifyTransaction(txn *types.Transaction) error {
	if err := CheckTransactionFee(txn); err != nil {
		return fmt.Errorf("[VerifyTransaction],%v\n", err)
	}

	if err := CheckTransactionNonce(txn); err != nil {
		return fmt.Errorf("[VerifyTransaction],%v\n", err)
	}

	if err := CheckTransactionAttribute(txn); err != nil {
		return fmt.Errorf("[VerifyTransaction],%v\n", err)
	}

	if err := CheckTransactionContracts(txn); err != nil {
		return fmt.Errorf("[VerifyTransaction],%v\n", err)
	}

	if err := CheckTransactionPayload(txn); err != nil {
		return fmt.Errorf("[VerifyTransaction],%v\n", err)
	}

	return nil
}

func CheckTransactionFee(txn *types.Transaction) error {
	// precise
	if checkAmountPrecise(Fixed64(txn.UnsignedTx.Fee), 8) {
		return errors.New("The precision of fee is incorrect.")
	}
	// amount
	if txn.UnsignedTx.Fee < 0 {
		return errors.New("tx fee error.")
	}

	return nil
}

func CheckTransactionNonce(txn *types.Transaction) error {
	return nil
}

func CheckTransactionAttribute(txn *types.Transaction) error {
	if len(txn.UnsignedTx.Attributes) > 100 {
		return errors.New("Attributes too long.")
	}
	return nil
}

func CheckTransactionContracts(txn *types.Transaction) error {
	flag, err := signature.VerifySignableData(txn)
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
	payload, err := types.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return err
	}

	switch txn.UnsignedTx.Payload.Type {
	case types.CoinbaseType:
		pld := payload.(*types.Coinbase)
		if len(pld.Sender) != 20 && len(pld.Recipient) != 20 {
			return errors.New("length of programhash error")
		}

		if BytesToUint160(pld.Sender) != EmptyUint160 {
			return errors.New("Sender error")
		}

		if checkAmountPrecise(Fixed64(pld.Amount), 8) {
			return errors.New("The precision of amount is incorrect.")
		}

		if Fixed64(pld.Amount) != Fixed64(config.DefaultMiningReward*StorageFactor) {
			return errors.New("Coinbase reward error.")
		}
	case types.TransferAssetType:
		pld := payload.(*types.TransferAsset)
		if len(pld.Sender) != 20 && len(pld.Recipient) != 20 {
			return errors.New("length of programhash error")
		}

		if checkAmountPrecise(Fixed64(pld.Amount), 8) {
			return errors.New("The precision of amount is incorrect.")
		}

		if pld.Amount < 0 {
			return errors.New("transfer amount error.")
		}
	case types.CommitType:
	case types.RegisterNameType:
		pld := payload.(*types.RegisterName)
		match, err := regexp.MatchString("([a-z]{8,12})", pld.Name)
		if err != nil {
			return err
		}
		if !match {
			return errors.New(fmt.Sprintf("name %s should only contain a-z and have length 8-12", pld.Name))
		}
	case types.DeleteNameType:
	case types.SubscribeType:
		pld := payload.(*types.Subscribe)
		duration := pld.Duration
		if duration > MaxSubscriptionDuration {
			return errors.New(fmt.Sprintf("subscription duration %d can't be bigger than %d", duration, MaxSubscriptionDuration))
		}

		topic := pld.Topic
		match, err := regexp.MatchString("(^[a-z][a-z0-9-_.~+%]{2,254}$)", topic)
		if err != nil {
			return err
		}
		if !match {
			return errors.New(fmt.Sprintf("topic %s should only contain a-z and have length 8-12", topic))
		}
	default:
		return errors.New("[txValidator],invalidate transaction payload type.")
	}
	return nil
}

// VerifyTransactionWithLedger verifys a transaction with history transaction in ledger
func VerifyTransactionWithLedger(txn *types.Transaction) error {
	if DefaultLedger.Store.IsDoubleSpend(txn) {
		return errors.New("[VerifyTransactionWithLedger] IsDoubleSpend check faild.")
	}

	if DefaultLedger.Store.IsTxHashDuplicate(txn.Hash()) {
		return errors.New("[VerifyTransactionWithLedger] duplicate transaction check faild.")
	}

	//TODO GetProgramHashes
	if txn.UnsignedTx.Payload.Type != types.CoinbaseType &&
		txn.UnsignedTx.Payload.Type != types.CommitType {
		addr, _ := ToCodeHash(txn.Programs[0].Code)
		nonce := DefaultLedger.Store.GetNonce(addr)
		if nonce != txn.UnsignedTx.Nonce {
			return errors.New("[VerifyTransactionWithLedger] txn nonce error.")
		}
	}

	payload, err := types.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return err
	}

	switch txn.UnsignedTx.Payload.Type {
	case types.CoinbaseType:
	case types.TransferAssetType:
		pld := payload.(*types.TransferAsset)
		balance := DefaultLedger.Store.GetBalance(BytesToUint160(pld.Sender))
		if int64(balance) < pld.Amount {
			return errors.New("not sufficient funds")
		}
	case types.CommitType:
	case types.RegisterNameType:
		pld := payload.(*types.RegisterName)
		name, err := DefaultLedger.Store.GetName(pld.Registrant)
		if name != nil {
			return errors.New(fmt.Sprintf("pubKey %+v already has registered name %s", pld.Registrant, *name))
		}
		if err != leveldb.ErrNotFound {
			return err
		}

		registrant, err := DefaultLedger.Store.GetRegistrant(pld.Name)
		if registrant != nil {
			return errors.New(fmt.Sprintf("name %s is already registered for pubKey %+v", pld.Name, registrant))
		}
		if err != leveldb.ErrNotFound {
			return err
		}
	case types.DeleteNameType:
		pld := payload.(*types.DeleteName)
		name, err := DefaultLedger.Store.GetName(pld.Registrant)
		if err != leveldb.ErrNotFound {
			return err
		}
		if name == nil {
			return errors.New(fmt.Sprintf("no name registered for pubKey %+v", pld.Registrant))
		}
	case types.SubscribeType:
		pld := payload.(*types.Subscribe)

		subscribed, err := DefaultLedger.Store.IsSubscribed(pld.Subscriber, pld.Identifier, pld.Topic)
		if err != nil {
			return err
		}
		if subscribed {
			return errors.New(fmt.Sprintf("subscriber %s already subscribed to %s", pld.SubscriberString(), pld.Topic))
		}

		subscriptionCount := DefaultLedger.Store.GetSubscribersCount(pld.Topic)
		if subscriptionCount >= SubscriptionsLimit {
			return errors.New(fmt.Sprintf("subscribtion count to %s can't be more than %d", pld.Topic, subscriptionCount))
		}
	default:
		return errors.New("[txValidator],invalidate transaction payload type.")
	}

	return nil
}

type Iterator interface {
	Iterate(handler func(item *types.Transaction) ErrCode) ErrCode
}

// VerifyTransactionWithBlock verifys a transaction with current transaction pool in memory
func VerifyTransactionWithBlock(iterator Iterator) ErrCode {
	//initial
	txnlist := make(map[Uint256]struct{}, 0)
	registeredNames := make(map[string]struct{}, 0)
	nameRegistrants := make(map[string]struct{}, 0)

	type subscription struct{ topic, subscriber string }
	subscriptions := make(map[subscription]struct{}, 0)

	subscriptionCount := make(map[string]int, 0)

	//start check
	return iterator.Iterate(func(txn *types.Transaction) ErrCode {
		//1.check weather have duplicate transaction.
		if _, exist := txnlist[txn.Hash()]; exist {
			log.Warning("[VerifyTransactionWithBlock], duplicate transaction exist in block.")
			return ErrDuplicatedTx
		} else {
			txnlist[txn.Hash()] = struct{}{}
		}

		//TODO check nonce duplicate

		//3.check issue amount
		payload, err := types.Unpack(txn.UnsignedTx.Payload)
		if err != nil {
			return ErrDuplicatedTx
		}

		switch txn.UnsignedTx.Payload.Type {
		case types.CoinbaseType:
			coinbase := payload.(*types.Coinbase)
			if Fixed64(coinbase.Amount) != Fixed64(config.DefaultMiningReward*StorageFactor) {
				log.Warning("Mining reward incorrectly.")
				return ErrMineReward
			}

		case types.RegisterNameType:
			namePayload := payload.(*types.RegisterName)

			name := namePayload.Name
			if _, ok := registeredNames[name]; ok {
				log.Warning("[VerifyTransactionWithBlock], duplicate name exist in block.")
				return ErrDuplicateName
			}
			registeredNames[name] = struct{}{}

			registrant := BytesToHexString(namePayload.Registrant)
			if _, ok := nameRegistrants[registrant]; ok {
				log.Warning("[VerifyTransactionWithBlock], duplicate registrant exist in block.")
				return ErrDuplicateName
			}
			nameRegistrants[registrant] = struct{}{}
		case types.DeleteNameType:
			namePayload := payload.(*types.DeleteName)

			registrant := BytesToHexString(namePayload.Registrant)
			if _, ok := nameRegistrants[registrant]; ok {
				log.Warning("[VerifyTransactionWithBlock], duplicate registrant exist in block.")
				return ErrDuplicateName
			}
			nameRegistrants[registrant] = struct{}{}
		case types.SubscribeType:
			subscribePayload := payload.(*types.Subscribe)
			topic := subscribePayload.Topic
			key := subscription{topic, subscribePayload.SubscriberString()}
			if _, ok := subscriptions[key]; ok {
				log.Warning("[VerifyTransactionWithBlock], duplicate subscription exist in block.")
				return ErrDuplicateSubscription
			}
			subscriptions[key] = struct{}{}

			if _, ok := subscriptionCount[topic]; !ok {
				subscriptionCount[topic] = DefaultLedger.Store.GetSubscribersCount(topic)
			}
			if subscriptionCount[topic] >= SubscriptionsLimit {
				log.Warning("[VerifyTransactionWithBlock], subscription limit exceeded in block.")
				return ErrSubscriptionLimit
			}
			subscriptionCount[topic]++
		}

		return ErrNoError
	})
}
