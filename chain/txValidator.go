package chain

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/config"
)

var (
	ErrIDRegistered           = errors.New("ID has be registered")
	ErrDuplicateGenerateIDTxn = errors.New("[VerifyTransactionWithBlock], duplicate GenerateID txns")
	ErrDuplicateIssueAssetTxn = errors.New("[VerifyTransactionWithBlock], duplicate IssueAsset txns")
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
	case pb.COINBASE_TYPE:
		pld := payload.(*pb.Coinbase)
		if len(pld.Sender) != UINT160SIZE && len(pld.Recipient) != UINT160SIZE {
			return errors.New("length of programhash error")
		}

		donationProgramhash, _ := ToScriptHash(config.DonationAddress)
		if BytesToUint160(pld.Sender) != donationProgramhash {
			return errors.New("Sender error")
		}

		if checkAmountPrecise(Fixed64(pld.Amount), 8) {
			return errors.New("The precision of amount is incorrect.")
		}
	case pb.TRANSFER_ASSET_TYPE:
		pld := payload.(*pb.TransferAsset)
		if len(pld.Sender) != UINT160SIZE && len(pld.Recipient) != UINT160SIZE {
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
	case pb.SIG_CHAIN_TXN_TYPE:
	case pb.REGISTER_NAME_TYPE:
		return errors.New("Register name transaction is not supported yet")
		// pld := payload.(*pb.RegisterName)
		// match, err := regexp.MatchString("(^[A-Za-z][A-Za-z0-9-_.+]{2,254}$)", pld.Name)
		// if err != nil {
		// 	return err
		// }
		// if !match {
		// 	return fmt.Errorf("name %s should start with a letter, contain A-Za-z0-9-_.+ and have length 3-255", pld.Name)
		// }
	case pb.DELETE_NAME_TYPE:
	case pb.SUBSCRIBE_TYPE:
		pld := payload.(*pb.Subscribe)

		if pld.Duration == 0 {
			return fmt.Errorf("duration can't be 0")
		}

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

		// Check sub.Meta & sub.Identifier limitation via height
		h := DefaultLedger.Store.GetHeight() + 1 // txn's height should be currHeight + 1
		if len(pld.Identifier) > config.MaxTxnSubIdentifierList.GetValueAtHeight(h) {
			return errors.New("Identifier too long")
		}
		if len(pld.Meta) > config.MaxTxnSubMetaList.GetValueAtHeight(h) {
			return errors.New("Meta too long")
		}
	case pb.GENERATE_ID_TYPE:
		pld := payload.(*pb.GenerateID)
		_, err := crypto.NewPubKeyFromBytes(pld.PublicKey)
		if err != nil {
			return fmt.Errorf("GenerateID error: %v", err)
		}

		if Fixed64(pld.RegistrationFee) < Fixed64(config.MinGenIDRegistrationFee) {
			return errors.New("fee is too low than MinGenIDRegistrationFee")
		}
	case pb.NANO_PAY_TYPE:
		pld := payload.(*pb.NanoPay)

		if len(pld.Sender) != UINT160SIZE && len(pld.Recipient) != UINT160SIZE {
			return errors.New("length of programhash error")
		}

		donationProgramhash, _ := ToScriptHash(config.DonationAddress)
		if bytes.Equal(pld.Sender, donationProgramhash[:]) {
			return errors.New("illegal transaction sender")
		}

		if checkAmountPrecise(Fixed64(pld.Amount), 8) {
			return errors.New("The precision of amount is incorrect")
		}

		if pld.Amount < 0 {
			return errors.New("transfer amount error")
		}

		if pld.TxnExpiration > pld.NanoPayExpiration {
			return errors.New("txn expiration should be no later than nano pay expiration")
		}

	case pb.ISSUE_ASSET_TYPE:
		pld := payload.(*pb.IssueAsset)
		if len(pld.Sender) != UINT160SIZE {
			return errors.New("length of programhash error")
		}

		match, err := regexp.MatchString("(^[A-Za-z][A-Za-z0-9 ]{2,11}$)", pld.Name)
		if err != nil {
			return err
		}
		if !match {
			return fmt.Errorf("name %s should start with a letter, contain A-Za-z0-9 and have length 3-12", pld.Name)
		}

		match, err = regexp.MatchString("(^[a-z][a-z0-9]{2,8}$)", pld.Symbol)
		if err != nil {
			return err
		}
		if !match {
			return fmt.Errorf("name %s should start with a letter, contain a-z0-9 and have length 3-9", pld.Name)
		}

		if pld.TotalSupply < 0 {
			return fmt.Errorf("TotalSupply %v should be a positive number", pld.TotalSupply)
		}

		if pld.Precision > config.MaxAssetPrecision {
			return fmt.Errorf("Precision %v should less than %v", pld.Precision, config.MaxAssetPrecision)
		}
	default:
		return fmt.Errorf("[txValidator], invalid transaction payload type %v", txn.UnsignedTx.Payload.Type)
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
		return errors.New("Unpack transactiion's payload error")
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
	case pb.COINBASE_TYPE:
		donationAmount, err := DefaultLedger.Store.GetDonation()
		if err != nil {
			return err
		}

		donationProgramhash, _ := ToScriptHash(config.DonationAddress)
		amount := DefaultLedger.Store.GetBalance(donationProgramhash)
		if amount < donationAmount {
			return errors.New("not sufficient funds in doation account")
		}
	case pb.TRANSFER_ASSET_TYPE:
		if err := checkNonce(); err != nil {
			return err
		}

		pld := payload.(*pb.TransferAsset)
		balance := DefaultLedger.Store.GetBalance(BytesToUint160(pld.Sender))
		if int64(balance) < pld.Amount {
			return errors.New("not sufficient funds")
		}
	case pb.SIG_CHAIN_TXN_TYPE:
	case pb.REGISTER_NAME_TYPE:
		if err := checkNonce(); err != nil {
			return err
		}

		pld := payload.(*pb.RegisterName)
		name, err := DefaultLedger.Store.GetName(pld.Registrant)
		if name != "" {
			return fmt.Errorf("pubKey %+v already has registered name %s", pld.Registrant, name)
		}
		if err != nil {
			return err
		}

		registrant, err := DefaultLedger.Store.GetRegistrant(pld.Name)
		if registrant != nil {
			return fmt.Errorf("name %s is already registered for pubKey %+v", pld.Name, registrant)
		}
		if err != nil {
			return err
		}
	case pb.DELETE_NAME_TYPE:
		if err := checkNonce(); err != nil {
			return err
		}

		pld := payload.(*pb.DeleteName)
		name, err := DefaultLedger.Store.GetName(pld.Registrant)
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("no name registered for pubKey %+v", pld.Registrant)
		} else if name != pld.Name {
			return fmt.Errorf("no name %s registered for pubKey %+v", pld.Name, pld.Registrant)
		}
	case pb.SUBSCRIBE_TYPE:
		if err := checkNonce(); err != nil {
			return err
		}

		pld := payload.(*pb.Subscribe)
		subscribed, err := DefaultLedger.Store.IsSubscribed(pld.Topic, pld.Bucket, pld.Subscriber, pld.Identifier)
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
	case pb.GENERATE_ID_TYPE:
		if err := checkNonce(); err != nil {
			return err
		}

		pld := payload.(*pb.GenerateID)
		id, err := DefaultLedger.Store.GetID(pld.PublicKey)
		if err != nil {
			return err
		}
		if len(id) != 0 {
			return ErrIDRegistered
		}
	case pb.NANO_PAY_TYPE:
		pld := payload.(*pb.NanoPay)

		channelBalance, _, err := DefaultLedger.Store.GetNanoPay(
			BytesToUint160(pld.Sender),
			BytesToUint160(pld.Recipient),
			pld.Id,
		)
		if err != nil {
			return err
		}

		height := DefaultLedger.Store.GetHeight()
		if height > pld.TxnExpiration {
			return errors.New("nano pay txn has expired")
		}
		if height > pld.NanoPayExpiration {
			return errors.New("nano pay has expired")
		}

		balance := DefaultLedger.Store.GetBalance(BytesToUint160(pld.Sender))
		balanceToClaim := pld.Amount - int64(channelBalance)
		if balanceToClaim <= 0 {
			return errors.New("invalid amount")
		}
		if int64(balance) < balanceToClaim {
			return errors.New("not sufficient funds")
		}

	case pb.ISSUE_ASSET_TYPE:
		if err := checkNonce(); err != nil {
			return err
		}

		assetID := txn.Hash()
		_, _, _, _, err := DefaultLedger.Store.GetAsset(assetID)
		if err == nil {
			return ErrDuplicateIssueAssetTxn
		}
	default:
		return fmt.Errorf("[txValidator], invalid transaction payload type %v", txn.UnsignedTx.Payload.Type)
	}
	return nil
}

type subscription struct {
	topic      string
	bucket     uint32
	subscriber string
	identifier string
}

type nanoPay struct {
	sender    string
	recipient string
	nonce     uint64
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
	nanoPays          map[nanoPay]struct{}

	changes []func()
}

func NewBlockValidationState() *BlockValidationState {
	bvs := &BlockValidationState{}
	bvs.initBlockValidationState()
	return bvs
}

func (bvs *BlockValidationState) initBlockValidationState() {
	bvs.txnlist = make(map[Uint256]struct{}, 0)
	bvs.totalAmount = make(map[Uint160]Fixed64, 0)
	bvs.registeredNames = make(map[string]struct{}, 0)
	bvs.nameRegistrants = make(map[string]struct{}, 0)
	bvs.generateIDs = make(map[string]struct{}, 0)
	bvs.subscriptions = make(map[subscription]struct{}, 0)
	bvs.subscriptionCount = make(map[string]int, 0)
	bvs.nanoPays = make(map[nanoPay]struct{}, 0)
}

func (bvs *BlockValidationState) addChange(change func()) {
	bvs.changes = append(bvs.changes, change)
}

func (bvs *BlockValidationState) Commit() {
	for _, change := range bvs.changes {
		change()
	}
	bvs.Reset()
}

func (bvs *BlockValidationState) Reset() {
	bvs.changes = nil
}

// VerifyTransactionWithBlock verifys a transaction with current transaction pool in memory
func (bvs *BlockValidationState) VerifyTransactionWithBlock(txn *transaction.Transaction, height uint32) (e error) {
	//1.check weather have duplicate transaction.
	if _, exist := bvs.txnlist[txn.Hash()]; exist {
		return errors.New("[VerifyTransactionWithBlock], duplicate transaction exist in block.")
	} else {
		defer func() {
			if e == nil {
				bvs.addChange(func() {
					bvs.txnlist[txn.Hash()] = struct{}{}
				})
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
	case pb.COINBASE_TYPE:
		coinbase := payload.(*pb.Coinbase)
		donationAmount, err := DefaultLedger.Store.GetDonation()
		if err != nil {
			return err
		}
		if Fixed64(coinbase.Amount) != GetRewardByHeight(height)+donationAmount {
			return errors.New("Mining reward incorrectly.")
		}
	case pb.TRANSFER_ASSET_TYPE:
		transfer := payload.(*pb.TransferAsset)
		amount = Fixed64(transfer.Amount)
	case pb.REGISTER_NAME_TYPE:
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
				bvs.addChange(func() {
					bvs.registeredNames[name] = struct{}{}
					bvs.nameRegistrants[registrant] = struct{}{}
				})
			}
		}()
	case pb.DELETE_NAME_TYPE:
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
				bvs.addChange(func() {
					bvs.registeredNames[name] = struct{}{}
					bvs.nameRegistrants[registrant] = struct{}{}
				})
			}
		}()
	case pb.SUBSCRIBE_TYPE:
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
				bvs.addChange(func() {
					bvs.subscriptions[key] = struct{}{}
					bvs.subscriptionCount[topic] = subscriptionCount + 1
				})
			}
		}()
	case pb.GENERATE_ID_TYPE:
		generateIdPayload := payload.(*pb.GenerateID)
		amount = Fixed64(generateIdPayload.RegistrationFee)
		publicKey := BytesToHexString(generateIdPayload.PublicKey)
		if _, ok := bvs.generateIDs[publicKey]; ok {
			return ErrDuplicateGenerateIDTxn
		}

		defer func() {
			if e == nil {
				bvs.addChange(func() {
					bvs.generateIDs[publicKey] = struct{}{}
				})
			}
		}()
	case pb.NANO_PAY_TYPE:
		npPayload := payload.(*pb.NanoPay)
		if height > npPayload.TxnExpiration {
			return errors.New("[VerifyTransactionWithBlock], nano pay txn has expired")
		}
		if height > npPayload.NanoPayExpiration {
			return errors.New("[VerifyTransactionWithBlock], nano pay has expired")
		}
		key := nanoPay{BytesToHexString(npPayload.Sender), BytesToHexString(npPayload.Recipient), npPayload.Id}
		if _, ok := bvs.nanoPays[key]; ok {
			return errors.New("[VerifyTransactionWithBlock], duplicate payment channel exist in block")
		}

		channelBalance, _, err := DefaultLedger.Store.GetNanoPay(
			BytesToUint160(npPayload.Sender),
			BytesToUint160(npPayload.Recipient),
			npPayload.Id,
		)
		if err != nil {
			return err
		}
		amount = Fixed64(npPayload.Amount) - channelBalance

		defer func() {
			if e == nil {
				bvs.addChange(func() {
					bvs.nanoPays[key] = struct{}{}
				})
			}
		}()

	case pb.ISSUE_ASSET_TYPE:
	}

	if amount > 0 || fee > 0 {
		balance := DefaultLedger.Store.GetBalance(sender)
		totalAmount := bvs.totalAmount[sender]
		if balance < totalAmount+amount+fee {
			return errors.New("[VerifyTransactionWithBlock], not sufficient funds.")
		}

		defer func() {
			if e == nil {
				bvs.addChange(func() {
					bvs.totalAmount[sender] = totalAmount + amount + fee
				})
			}
		}()
	}

	return nil
}

func (bvs *BlockValidationState) CleanSubmittedTransactions(txns []*transaction.Transaction) error {
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
		case pb.TRANSFER_ASSET_TYPE:
			transfer := payload.(*pb.TransferAsset)
			amount = Fixed64(transfer.Amount)
		case pb.REGISTER_NAME_TYPE:
			namePayload := payload.(*pb.RegisterName)

			name := namePayload.Name
			delete(bvs.registeredNames, name)

			registrant := BytesToHexString(namePayload.Registrant)
			delete(bvs.nameRegistrants, registrant)
		case pb.DELETE_NAME_TYPE:
			namePayload := payload.(*pb.DeleteName)

			name := namePayload.Name
			delete(bvs.registeredNames, name)

			registrant := BytesToHexString(namePayload.Registrant)
			delete(bvs.nameRegistrants, registrant)
		case pb.SUBSCRIBE_TYPE:
			subscribePayload := payload.(*pb.Subscribe)
			topic := subscribePayload.Topic
			bucket := subscribePayload.Bucket
			key := subscription{topic, bucket, BytesToHexString(subscribePayload.Subscriber), subscribePayload.Identifier}
			delete(bvs.subscriptions, key)

			bvs.subscriptionCount[topic]--

			if bvs.subscriptionCount[topic] == 0 {
				delete(bvs.subscriptionCount, topic)
			}
		case pb.GENERATE_ID_TYPE:
			generateIdPayload := payload.(*pb.GenerateID)
			amount = Fixed64(generateIdPayload.RegistrationFee)
			publicKey := BytesToHexString(generateIdPayload.PublicKey)
			delete(bvs.generateIDs, publicKey)
		case pb.NANO_PAY_TYPE:
			npPayload := payload.(*pb.NanoPay)
			key := nanoPay{BytesToHexString(npPayload.Sender), BytesToHexString(npPayload.Recipient), npPayload.Id}
			delete(bvs.nanoPays, key)
		case pb.ISSUE_ASSET_TYPE:
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

func (bvs *BlockValidationState) RefreshBlockValidationState(txns []*transaction.Transaction) error {
	bvs.initBlockValidationState()
	for _, tx := range txns {
		if err := bvs.VerifyTransactionWithBlock(tx, 0); err != nil {
			return err
		}
	}

	return nil
}
