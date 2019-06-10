package chain

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sync"

	"github.com/nknorg/nkn/block"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/config"
	"github.com/syndtr/goleveldb/leveldb"
)

// VerifyTransaction verifys received single transaction
func VerifyTransaction(txn *transaction.Transaction) error {
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

	if err := txn.VerifySignature(); err != nil {
		return fmt.Errorf("[VerifyTransaction],%v\n", err)
	}

	if err := CheckTransactionPayload(txn); err != nil {
		return fmt.Errorf("[VerifyTransaction],%v\n", err)
	}

	return nil
}

func CheckTransactionSize(txn *transaction.Transaction) error {
	size := txn.GetSize()
	if size <= 0 || size > config.MaxBlockSize {
		return fmt.Errorf("Invalid transaction size: %d bytes", size)
	}

	return nil
}

func CheckTransactionFee(txn *transaction.Transaction) error {
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

func CheckTransactionNonce(txn *transaction.Transaction) error {
	return nil
}

func CheckTransactionAttribute(txn *transaction.Transaction) error {
	if len(txn.UnsignedTx.Attributes) > 100 {
		return errors.New("Attributes too long.")
	}
	return nil
}

func checkAmountPrecise(amount Fixed64, precision byte) bool {
	return amount.GetData()%int64(math.Pow(10, 8-float64(precision))) != 0
}

func CheckTransactionPayload(txn *transaction.Transaction) error {
	payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return err
	}

	switch txn.UnsignedTx.Payload.Type {
	case pb.CoinbaseType:
		pld := payload.(*pb.Coinbase)
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
	case pb.TransferAssetType:
		pld := payload.(*pb.TransferAsset)
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
	case pb.CommitType:
	case pb.RegisterNameType:
		pld := payload.(*pb.RegisterName)
		match, err := regexp.MatchString("(^[A-Za-z][A-Za-z0-9-_.+]{2,254}$)", pld.Name)
		if err != nil {
			return err
		}
		if !match {
			return fmt.Errorf("name %s should start with a letter, contain A-Za-z0-9-_.+ and have length 3-255", pld.Name)
		}
	case pb.DeleteNameType:
	case pb.SubscribeType:
		pld := payload.(*pb.Subscribe)
		bucket := pld.Bucket
		if bucket > transaction.BucketsLimit {
			return fmt.Errorf("topic bucket %d can't be bigger than %d", bucket, transaction.BucketsLimit)
		}

		duration := pld.Duration
		if duration > transaction.MaxSubscriptionDuration {
			return fmt.Errorf("subscription duration %d can't be bigger than %d", duration, transaction.MaxSubscriptionDuration)
		}

		topic := pld.Topic
		match, err := regexp.MatchString("(^[A-Za-z][A-Za-z0-9-_.+]{2,254}$)", topic)
		if err != nil {
			return err
		}
		if !match {
			return fmt.Errorf("topic %s should start with a letter, contain A-Za-z0-9-_.+ and have length 3-255", topic)
		}
	case pb.GenerateIDType:
		pld := payload.(*pb.GenerateID)
		_, err := crypto.NewPubKeyFromBytes(pld.PublicKey)
		if err != nil {
			return fmt.Errorf("GenerateID error: %v", err)
		}

		if Fixed64(pld.RegistrationFee) < Fixed64(config.MinGenIDRegistrationFee) {
			return errors.New("fee is too low than MinGenIDRegistrationFee")
		}

	default:
		return errors.New("[txValidator],invalidate transaction payload type")
	}
	return nil
}

// VerifyTransactionWithLedger verifys a transaction with history transaction in ledger
func VerifyTransactionWithLedger(txn *transaction.Transaction) error {
	if DefaultLedger.Store.IsDoubleSpend(txn) {
		return errors.New("[VerifyTransactionWithLedger] IsDoubleSpend check faild")
	}

	if DefaultLedger.Store.IsTxHashDuplicate(txn.Hash()) {
		return errors.New("[VerifyTransactionWithLedger] duplicate transaction check faild")
	}

	payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return errors.New("Unpack transactiion's paylaod error")
	}

	checkNonce := func() error {
		sender, err := ToCodeHash(txn.Programs[0].Code)
		if err != nil {
			return err
		}
		nonce := DefaultLedger.Store.GetNonce(sender)

		if txn.UnsignedTx.Nonce < nonce {
			return errors.New("nonce is too low")
		}

		return nil
	}

	switch txn.UnsignedTx.Payload.Type {
	case pb.CoinbaseType:
		donationAmount, err := DefaultLedger.Store.GetDonation()
		if err != nil {
			return err
		}

		donationProgramhash, _ := ToScriptHash(config.DonationAddress)
		amount := DefaultLedger.Store.GetBalance(donationProgramhash)
		if amount < donationAmount {
			return errors.New("not sufficient funds in doation account")
		}
	case pb.TransferAssetType:
		if err := checkNonce(); err != nil {
			return err
		}

		pld := payload.(*pb.TransferAsset)
		balance := DefaultLedger.Store.GetBalance(BytesToUint160(pld.Sender))
		if int64(balance) < pld.Amount {
			return errors.New("not sufficient funds")
		}
	case pb.CommitType:
	case pb.RegisterNameType:
		if err := checkNonce(); err != nil {
			return err
		}

		pld := payload.(*pb.RegisterName)
		name, err := DefaultLedger.Store.GetName(pld.Registrant)
		if name != nil {
			return fmt.Errorf("pubKey %+v already has registered name %s", pld.Registrant, *name)
		}
		if err != leveldb.ErrNotFound {
			return err
		}

		registrant, err := DefaultLedger.Store.GetRegistrant(pld.Name)
		if registrant != nil {
			return fmt.Errorf("name %s is already registered for pubKey %+v", pld.Name, registrant)
		}
		if err != leveldb.ErrNotFound {
			return err
		}
	case pb.DeleteNameType:
		if err := checkNonce(); err != nil {
			return err
		}

		pld := payload.(*pb.DeleteName)
		name, err := DefaultLedger.Store.GetName(pld.Registrant)
		if err != leveldb.ErrNotFound {
			return err
		}
		if name == nil {
			return fmt.Errorf("no name registered for pubKey %+v", pld.Registrant)
		} else if *name != pld.Name {
			return fmt.Errorf("no name %s registered for pubKey %+v", pld.Name, pld.Registrant)
		}
	case pb.SubscribeType:
		if err := checkNonce(); err != nil {
			return err
		}

		pld := payload.(*pb.Subscribe)
		subscribed, err := DefaultLedger.Store.IsSubscribed(pld.Subscriber, pld.Identifier, pld.Topic, pld.Bucket)
		if err != nil {
			return err
		}
		if subscribed {
			return fmt.Errorf("subscriber %s already subscribed to %s", address.MakeAddressString(pld.Subscriber, pld.Identifier), pld.Topic)
		}

		subscriptionCount := DefaultLedger.Store.GetSubscribersCount(pld.Topic, pld.Bucket)
		if subscriptionCount >= transaction.SubscriptionsLimit {
			return fmt.Errorf("subscribtion count to %s can't be more than %d", pld.Topic, subscriptionCount)
		}
	case pb.GenerateIDType:
		if err := checkNonce(); err != nil {
			return err
		}

		pld := payload.(*pb.GenerateID)
		id, err := DefaultLedger.Store.GetID(pld.PublicKey)
		if err != nil {
			return err
		}
		if len(id) != 0 {
			return errors.New("ID has be registered")
		}
	default:
		return errors.New("[txValidator],invalidate transaction payload type.")
	}
	return nil
}

type subscription struct {
	topic      string
	bucket     uint32
	subscriber string
	identifier string
}

type BlockValidationState struct {
	sync.Mutex
	txnlist           map[Uint256]struct{}
	totalAmount       map[Uint160]Fixed64
	registeredNames   map[string]struct{}
	nameRegistrants   map[string]struct{}
	generateIDs       map[string]struct{}
	subscriptions     map[subscription]struct{}
	subscriptionCount map[string]int
}

func NewBlockValidationState() *BlockValidationState {
	return &BlockValidationState{
		txnlist:           make(map[Uint256]struct{}, 0),
		totalAmount:       make(map[Uint160]Fixed64, 0),
		registeredNames:   make(map[string]struct{}, 0),
		nameRegistrants:   make(map[string]struct{}, 0),
		generateIDs:       make(map[string]struct{}, 0),
		subscriptions:     make(map[subscription]struct{}, 0),
		subscriptionCount: make(map[string]int, 0),
	}
}

// VerifyTransactionWithBlock verifys a transaction with current transaction pool in memory
func (bvs *BlockValidationState) VerifyTransactionWithBlock(txn *transaction.Transaction, header *block.Header) (e error) {
	bvs.Lock()
	defer bvs.Unlock()
	//1.check weather have duplicate transaction.
	if _, exist := bvs.txnlist[txn.Hash()]; exist {
		return errors.New("[VerifyTransactionWithBlock], duplicate transaction exist in block.")
	} else {
		defer func() {
			if e == nil {
				bvs.txnlist[txn.Hash()] = struct{}{}
			}
		}()
	}

	//3.check issue amount
	payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return errors.New("[VerifyTransactionWithBlock], payload unpack error.")
	}

	pg, err := txn.GetProgramHashes()
	if err != nil {
		return err
	}
	sender := pg[0]
	var amount Fixed64
	fee := Fixed64(txn.UnsignedTx.Fee)

	switch txn.UnsignedTx.Payload.Type {
	case pb.CoinbaseType:
		if header != nil {
			coinbase := payload.(*pb.Coinbase)
			donationAmount, err := DefaultLedger.Store.GetDonation()
			if err != nil {
				return err
			}
			if Fixed64(coinbase.Amount) != GetRewardByHeight(header.UnsignedHeader.Height)+donationAmount {
				return errors.New("Mining reward incorrectly.")
			}
		}
	case pb.TransferAssetType:
		transfer := payload.(*pb.TransferAsset)
		amount = Fixed64(transfer.Amount)
	case pb.RegisterNameType:
		namePayload := payload.(*pb.RegisterName)

		name := namePayload.Name
		if _, ok := bvs.registeredNames[name]; ok {
			return errors.New("[VerifyTransactionWithBlock], duplicate name exist in block.")
		}

		registrant := BytesToHexString(namePayload.Registrant)
		if _, ok := bvs.nameRegistrants[registrant]; ok {
			return errors.New("[VerifyTransactionWithBlock], duplicate registrant exist in block.")
		}

		defer func() {
			if e == nil {
				bvs.registeredNames[name] = struct{}{}
				bvs.nameRegistrants[registrant] = struct{}{}
			}
		}()
	case pb.DeleteNameType:
		namePayload := payload.(*pb.DeleteName)

		name := namePayload.Name
		if _, ok := bvs.registeredNames[name]; ok {
			return errors.New("[VerifyTransactionWithBlock], duplicate name exist in block.")
		}

		registrant := BytesToHexString(namePayload.Registrant)
		if _, ok := bvs.nameRegistrants[registrant]; ok {
			return errors.New("[VerifyTransactionWithBlock], duplicate registrant exist in block.")
		}

		defer func() {
			if e == nil {
				bvs.registeredNames[name] = struct{}{}
				bvs.nameRegistrants[registrant] = struct{}{}
			}
		}()
	case pb.SubscribeType:
		subscribePayload := payload.(*pb.Subscribe)
		topic := subscribePayload.Topic
		bucket := subscribePayload.Bucket
		key := subscription{topic, bucket, BytesToHexString(subscribePayload.Subscriber), subscribePayload.Identifier}
		if _, ok := bvs.subscriptions[key]; ok {
			return errors.New("[VerifyTransactionWithBlock], duplicate subscription exist in block")
		}

		subscriptionCount := bvs.subscriptionCount[topic]
		ledgerSubscriptionCount := DefaultLedger.Store.GetSubscribersCount(topic, bucket)
		if ledgerSubscriptionCount+subscriptionCount >= transaction.SubscriptionsLimit {
			return errors.New("[VerifyTransactionWithBlock], subscription limit exceeded in block.")
		}

		defer func() {
			if e == nil {
				bvs.subscriptions[key] = struct{}{}
				bvs.subscriptionCount[topic] = subscriptionCount + 1
			}
		}()
	case pb.GenerateIDType:
		generateIdPayload := payload.(*pb.GenerateID)
		amount = Fixed64(generateIdPayload.RegistrationFee)
		publicKey := BytesToHexString(generateIdPayload.PublicKey)
		if _, ok := bvs.generateIDs[publicKey]; ok {
			return errors.New("[VerifyTransactionWithBlock], duplicate GenerateID txns in block.")
		}

		defer func() {
			if e == nil {
				bvs.generateIDs[publicKey] = struct{}{}
			}
		}()
	}

	if amount > 0 || fee > 0 {
		balance := DefaultLedger.Store.GetBalance(sender)
		totalAmount := bvs.totalAmount[sender]
		if balance < totalAmount+amount+fee {
			return errors.New("[VerifyTransactionWithBlock], not sufficient funds.")
		}

		defer func() {
			if e == nil {
				bvs.totalAmount[sender] = totalAmount + amount + fee
			}
		}()
	}

	return nil
}

func (bvs *BlockValidationState) CleanSubmittedTransactions(txns []*transaction.Transaction) error {
	bvs.Lock()
	defer bvs.Unlock()
	for _, txn := range txns {
		delete(bvs.txnlist, txn.Hash())

		payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
		if err != nil {
			return errors.New("[CleanSubmittedTransactions], payload unpack error.")
		}

		pg, err := txn.GetProgramHashes()
		if err != nil {
			return err
		}
		sender := pg[0]
		var amount Fixed64
		fee := Fixed64(txn.UnsignedTx.Fee)

		switch txn.UnsignedTx.Payload.Type {
		case pb.TransferAssetType:
			transfer := payload.(*pb.TransferAsset)
			amount = Fixed64(transfer.Amount)
		case pb.RegisterNameType:
			namePayload := payload.(*pb.RegisterName)

			name := namePayload.Name
			delete(bvs.registeredNames, name)

			registrant := BytesToHexString(namePayload.Registrant)
			delete(bvs.nameRegistrants, registrant)
		case pb.DeleteNameType:
			namePayload := payload.(*pb.DeleteName)

			name := namePayload.Name
			delete(bvs.registeredNames, name)

			registrant := BytesToHexString(namePayload.Registrant)
			delete(bvs.nameRegistrants, registrant)
		case pb.SubscribeType:
			subscribePayload := payload.(*pb.Subscribe)
			topic := subscribePayload.Topic
			bucket := subscribePayload.Bucket
			key := subscription{topic, bucket, BytesToHexString(subscribePayload.Subscriber), subscribePayload.Identifier}
			delete(bvs.subscriptions, key)

			bvs.subscriptionCount[topic]--

			if bvs.subscriptionCount[topic] == 0 {
				delete(bvs.subscriptionCount, topic)
			}
		case pb.GenerateIDType:
			generateIdPayload := payload.(*pb.GenerateID)
			amount = Fixed64(generateIdPayload.RegistrationFee)
			publicKey := BytesToHexString(generateIdPayload.PublicKey)
			delete(bvs.generateIDs, publicKey)
		}

		if amount > 0 || fee > 0 {
			if _, ok := bvs.totalAmount[sender]; ok && bvs.totalAmount[sender] >= amount+fee {
				bvs.totalAmount[sender] -= amount + fee

				if bvs.totalAmount[sender] == 0 {
					delete(bvs.totalAmount, sender)
				}
			} else {
				return errors.New("[CleanSubmittedTransactions], inconsistent block validation state.")
			}
		}
	}

	return nil
}
