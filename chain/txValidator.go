package chain

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"regexp"

	"github.com/nknorg/nkn/block"
	. "github.com/nknorg/nkn/common"
	. "github.com/nknorg/nkn/pb"
	. "github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/vm/signature"
	"github.com/syndtr/goleveldb/leveldb"
)

// VerifyTransaction verifys received single transaction
func VerifyTransaction(txn *Transaction) error {
	if err := CheckTransactionSize(txn); err != nil {
		return fmt.Errorf("[VerifyTransaction],%v\n", err)
	}

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

func CheckTransactionSize(txn *Transaction) error {
	size := txn.GetSize()
	if size <= 0 || size > config.MaxBlockSize {
		return fmt.Errorf("Invalid transaction size: %d bytes", size)
	}

	return nil
}

func CheckTransactionFee(txn *Transaction) error {
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

func CheckTransactionNonce(txn *Transaction) error {
	return nil
}

func CheckTransactionAttribute(txn *Transaction) error {
	if len(txn.UnsignedTx.Attributes) > 100 {
		return errors.New("Attributes too long.")
	}
	return nil
}

func CheckTransactionContracts(txn *Transaction) error {
	if txn.UnsignedTx.Payload.Type == CoinbaseType {
		return nil
	}

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

func CheckTransactionPayload(txn *Transaction) error {
	payload, err := Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return err
	}

	switch txn.UnsignedTx.Payload.Type {
	case CoinbaseType:
		pld := payload.(*Coinbase)
		if len(pld.Sender) != 20 && len(pld.Recipient) != 20 {
			return errors.New("length of programhash error")
		}

		donationProgramhash, _ := ToScriptHash(config.DonationAddress)
		if BytesToUint160(pld.Sender) != donationProgramhash {
			return errors.New("Sender error")
		}

		if checkAmountPrecise(Fixed64(pld.Amount), 8) {
			return errors.New("The precision of amount is incorrect.")
		}
	case TransferAssetType:
		pld := payload.(*TransferAsset)
		if len(pld.Sender) != 20 && len(pld.Recipient) != 20 {
			return errors.New("length of programhash error")
		}

		donationProgramhash, _ := ToScriptHash(config.DonationAddress)
		if bytes.Equal(pld.Sender, donationProgramhash[:]) {
			return errors.New("illegal transaction sender")
		}

		if checkAmountPrecise(Fixed64(pld.Amount), 8) {
			return errors.New("The precision of amount is incorrect.")
		}

		if pld.Amount < 0 {
			return errors.New("transfer amount error.")
		}
	case CommitType:
	case RegisterNameType:
		pld := payload.(*RegisterName)
		match, err := regexp.MatchString("([a-z]{8,12})", pld.Name)
		if err != nil {
			return err
		}
		if !match {
			return errors.New(fmt.Sprintf("name %s should only contain a-z and have length 8-12", pld.Name))
		}
	case DeleteNameType:
	case SubscribeType:
		pld := payload.(*Subscribe)
		subscribed, err := DefaultLedger.Store.IsSubscribed(pld.Subscriber, pld.Identifier, pld.Topic, pld.Bucket)
		if err != nil {
			return err
		}
		if subscribed {
			return errors.New(fmt.Sprintf("subscriber %s already subscribed to %s", SubscriberString(pld), pld.Topic))
		}

		subscriptionCount := DefaultLedger.Store.GetSubscribersCount(pld.Topic, pld.Bucket)
		if subscriptionCount >= SubscriptionsLimit {
			return errors.New(fmt.Sprintf("subscribtion count to %s can't be more than %d", pld.Topic, subscriptionCount))
		}
	default:
		return errors.New("[txValidator],invalidate transaction payload type")
	}
	return nil
}

// VerifyTransactionWithLedger verifys a transaction with history transaction in ledger
func VerifyTransactionWithLedger(txn *Transaction) error {
	if DefaultLedger.Store.IsDoubleSpend(txn) {
		return errors.New("[VerifyTransactionWithLedger] IsDoubleSpend check faild")
	}

	if DefaultLedger.Store.IsTxHashDuplicate(txn.Hash()) {
		return errors.New("[VerifyTransactionWithLedger] duplicate transaction check faild")
	}

	//TODO GetProgramHashes
	if txn.UnsignedTx.Payload.Type != CoinbaseType &&
		txn.UnsignedTx.Payload.Type != CommitType {
		addr, _ := ToCodeHash(txn.Programs[0].Code)
		nonce := DefaultLedger.Store.GetNonce(addr)
		if nonce != txn.UnsignedTx.Nonce {
			return fmt.Errorf("[VerifyTransactionWithLedger] txn nonce error, expected: %v, Get: %v", nonce, txn.UnsignedTx.Nonce)
		}
	}

	payload, err := Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return errors.New("Unpack transactiion's paylaod error")
	}

	switch txn.UnsignedTx.Payload.Type {
	case CoinbaseType:
		donation, err := DefaultLedger.Store.GetDonation()
		if err != nil {
			return err
		}

		donationProgramhash, _ := ToScriptHash(config.DonationAddress)
		amount := DefaultLedger.Store.GetBalance(donationProgramhash)
		if amount < donation.Amount {
			return errors.New("not sufficient funds in doation account")
		}
	case TransferAssetType:
		pld := payload.(*TransferAsset)
		balance := DefaultLedger.Store.GetBalance(BytesToUint160(pld.Sender))
		if int64(balance) < pld.Amount {
			return errors.New("not sufficient funds")
		}
	case CommitType:
	case RegisterNameType:
		pld := payload.(*RegisterName)
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
	case DeleteNameType:
		pld := payload.(*DeleteName)
		name, err := DefaultLedger.Store.GetName(pld.Registrant)
		if err != leveldb.ErrNotFound {
			return err
		}
		if *name != pld.Name {
			return errors.New(fmt.Sprintf("no name %s registered for pubKey %+v", pld.Name, pld.Registrant))
		} else if name == nil {
			return errors.New(fmt.Sprintf("no name registered for pubKey %+v", pld.Registrant))
		}
	case SubscribeType:
		pld := payload.(*Subscribe)
		bucket := pld.Bucket
		if bucket > BucketsLimit {
			return errors.New(fmt.Sprintf("topic bucket %d can't be bigger than %d", bucket, BucketsLimit))
		}

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
			return errors.New(fmt.Sprintf("topic %s should start with a-z, contain a-z0-9-_.~+%% and have length 3-255", topic))
		}

		subscribed, err := DefaultLedger.Store.IsSubscribed(pld.Subscriber, pld.Identifier, topic, bucket)
		if err != nil {
			return err
		}
		if subscribed {
			return fmt.Errorf("subscriber %s already subscribed to %s", SubscriberString(pld), topic)
		}

		subscriptionCount := DefaultLedger.Store.GetSubscribersCount(topic, bucket)
		if subscriptionCount >= SubscriptionsLimit {
			return fmt.Errorf("subscribtion count to %s can't be more than %d", topic, subscriptionCount)
		}
	default:
		return errors.New("[txValidator],invalidate transaction payload type.")
	}
	return nil
}

type Iterator interface {
	Iterate(handler func(item *Transaction) error) error
}

// VerifyTransactionWithBlock verifys a transaction with current transaction pool in memory
func VerifyTransactionWithBlock(iterator Iterator, header *block.Header) error {
	//initial
	txnlist := make(map[Uint256]struct{}, 0)
	registeredNames := make(map[string]struct{}, 0)
	nameRegistrants := make(map[string]struct{}, 0)

	type subscription struct{ topic, subscriber string }
	subscriptions := make(map[subscription]struct{}, 0)

	subscriptionCount := make(map[string]int, 0)

	//start check
	return iterator.Iterate(func(txn *Transaction) error {
		//1.check weather have duplicate transaction.
		if _, exist := txnlist[txn.Hash()]; exist {
			return errors.New("[VerifyTransactionWithBlock], duplicate transaction exist in block.")
		} else {
			txnlist[txn.Hash()] = struct{}{}
		}

		//TODO check nonce duplicate

		//3.check issue amount
		payload, err := Unpack(txn.UnsignedTx.Payload)
		if err != nil {
			return errors.New("[VerifyTransactionWithBlock], duplicate transaction exist in block.")
		}

		switch txn.UnsignedTx.Payload.Type {
		case CoinbaseType:
			coinbase := payload.(*Coinbase)
			donation, err := DefaultLedger.Store.GetDonation()
			if err != nil {
				return err
			}
			if Fixed64(coinbase.Amount) != GetRewardByHeight(header.UnsignedHeader.Height)+donation.Amount {
				return errors.New("Mining reward incorrectly.")
			}
		case RegisterNameType:
			namePayload := payload.(*RegisterName)

			name := namePayload.Name
			if _, ok := registeredNames[name]; ok {
				return errors.New("[VerifyTransactionWithBlock], duplicate name exist in block.")
			}
			registeredNames[name] = struct{}{}

			registrant := BytesToHexString(namePayload.Registrant)
			if _, ok := nameRegistrants[registrant]; ok {
				return errors.New("[VerifyTransactionWithBlock], duplicate registrant exist in block.")
			}
			nameRegistrants[registrant] = struct{}{}
		case DeleteNameType:
			namePayload := payload.(*DeleteName)

			registrant := BytesToHexString(namePayload.Registrant)
			if _, ok := nameRegistrants[registrant]; ok {
				return errors.New("[VerifyTransactionWithBlock], duplicate registrant exist in block.")
			}
			nameRegistrants[registrant] = struct{}{}
		case SubscribeType:
			subscribePayload := payload.(*Subscribe)
			topic := subscribePayload.Topic
			key := subscription{topic, SubscriberString(subscribePayload)}
			if _, ok := subscriptions[key]; ok {
				return errors.New("[VerifyTransactionWithBlock], duplicate subscription exist in block")
			}
			subscriptions[key] = struct{}{}

			if _, ok := subscriptionCount[topic]; !ok {
				subscriptionCount[topic] = DefaultLedger.Store.GetSubscribersCount(topic, subscribePayload.Bucket)
			}
			if subscriptionCount[topic] >= SubscriptionsLimit {
				return errors.New("[VerifyTransactionWithBlock], subscription limit exceeded in block.")
			}
			subscriptionCount[topic]++
		}

		return nil
	})

}

func SubscriberString(s *Subscribe) string {
	return address.MakeAddressString(s.Subscriber, s.Identifier)
}
