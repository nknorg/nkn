package chain

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sync"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto/ed25519"

	"github.com/nknorg/nkn/program"

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
func VerifyTransaction(txn *transaction.Transaction, height uint32) error {
	if err := CheckTransactionSize(txn); err != nil {
		return fmt.Errorf("[VerifyTransaction] %v", err)
	}

	if err := CheckAmount(txn.UnsignedTx.Fee); err != nil {
		return fmt.Errorf("[VerifyTransaction] fee %v", err)
	}

	if err := CheckTransactionNonce(txn); err != nil {
		return fmt.Errorf("[VerifyTransaction] %v", err)
	}

	if err := CheckTransactionAttribute(txn); err != nil {
		return fmt.Errorf("[VerifyTransaction] %v", err)
	}

	if err := txn.VerifySignature(); err != nil {
		return fmt.Errorf("[VerifyTransaction] %v", err)
	}

	if err := CheckTransactionPayload(txn, height); err != nil {
		return fmt.Errorf("[VerifyTransaction] %v", err)
	}

	return nil
}

func CheckTransactionSize(txn *transaction.Transaction) error {
	size := txn.GetSize()
	if size <= 0 || size > config.MaxBlockSize {
		return fmt.Errorf("invalid transaction size: %d bytes", size)
	}

	return nil
}

func CheckAmount(amount int64) error {
	if amount < 0 {
		return fmt.Errorf("amount %d is less than 0", amount)
	}

	if amount > config.InitialIssueAmount+config.TotalMiningRewards {
		return fmt.Errorf("amount %d is greater than max supply", amount)
	}

	return nil
}

func CheckTransactionNonce(txn *transaction.Transaction) error {
	return nil
}

func CheckTransactionAttribute(txn *transaction.Transaction) error {
	maxAttrsLen := config.MaxTxnAttributesLen
	if len(txn.UnsignedTx.Attributes) > maxAttrsLen {
		return fmt.Errorf("attributes len %d is greater than %d", len(txn.UnsignedTx.Attributes), maxAttrsLen)
	}
	return nil
}

func verifyPubSubTopic(topic string, height uint32) error {
	regexPattern := config.AllowSubscribeTopicRegex.GetValueAtHeight(height)
	match, err := regexp.MatchString(regexPattern, topic)
	if err != nil {
		return err
	}
	if !match {
		return fmt.Errorf("topic %s should match %s", topic, regexPattern)
	}
	return nil

}

func CheckTransactionPayload(txn *transaction.Transaction, height uint32) error {
	payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return err
	}

	switch txn.UnsignedTx.Payload.Type {
	case pb.COINBASE_TYPE:
		pld := payload.(*pb.Coinbase)
		if len(pld.Sender) != common.UINT160SIZE && len(pld.Recipient) != common.UINT160SIZE {
			return errors.New("length of programhash error")
		}

		donationProgramhash, _ := common.ToScriptHash(config.DonationAddress)
		if common.BytesToUint160(pld.Sender) != donationProgramhash {
			return errors.New("invalid sender")
		}

		if err = CheckAmount(pld.Amount); err != nil {
			return err
		}
	case pb.TRANSFER_ASSET_TYPE:
		pld := payload.(*pb.TransferAsset)
		if len(pld.Sender) != common.UINT160SIZE && len(pld.Recipient) != common.UINT160SIZE {
			return errors.New("length of programhash error")
		}

		donationProgramhash, _ := common.ToScriptHash(config.DonationAddress)
		if bytes.Equal(pld.Sender, donationProgramhash[:]) {
			return errors.New("illegal transaction sender")
		}

		if err = CheckAmount(pld.Amount); err != nil {
			return err
		}
	case pb.SIG_CHAIN_TXN_TYPE:
	case pb.REGISTER_NAME_TYPE:
		if ok := config.AllowTxnRegisterName.GetValueAtHeight(height); !ok {
			return errors.New("Register name transaction is not supported yet")
		}

		pld := payload.(*pb.RegisterName)
		if !config.LegacyNameService.GetValueAtHeight(height) {
			if err = CheckAmount(pld.RegistrationFee); err != nil {
				return err
			}
			if common.Fixed64(pld.RegistrationFee) < common.Fixed64(config.MinNameRegistrationFee) {
				return fmt.Errorf("registration fee %s is lower than MinNameRegistrationFee %d", string(pld.Registrant), config.MinNameRegistrationFee)
			}
		}
		regexPattern := config.AllowNameRegex.GetValueAtHeight(height)
		match, err := regexp.MatchString(regexPattern, pld.Name)
		if err != nil {
			return err
		}
		if !match {
			return fmt.Errorf("name %s should match regex %s", pld.Name, regexPattern)
		}
	case pb.TRANSFER_NAME_TYPE:
		pld := payload.(*pb.TransferName)
		if len(pld.Registrant) != ed25519.PublicKeySize {
			return fmt.Errorf("registrant invalid")
		}
	case pb.DELETE_NAME_TYPE:
		pld := payload.(*pb.DeleteName)
		if len(pld.Registrant) != ed25519.PublicKeySize {
			return fmt.Errorf("registrant invalid")
		}
	case pb.SUBSCRIBE_TYPE:
		pld := payload.(*pb.Subscribe)

		if pld.Duration == 0 {
			return fmt.Errorf("subscribe duration should be greater than 0")
		}

		maxSubscribeBucket := config.MaxSubscribeBucket.GetValueAtHeight(height)
		if pld.Bucket > uint32(maxSubscribeBucket) {
			return fmt.Errorf("subscribe bucket %d is greater than %d", pld.Bucket, maxSubscribeBucket)
		}

		maxDuration := config.MaxSubscribeDuration.GetValueAtHeight(height)
		if pld.Duration > uint32(maxDuration) {
			return fmt.Errorf("subscribe duration %d is greater than %d", pld.Duration, maxDuration)
		}

		if err = verifyPubSubTopic(pld.Topic, height); err != nil {
			return err
		}

		maxIdentifierLen := config.MaxSubscribeIdentifierLen.GetValueAtHeight(height)
		if len(pld.Identifier) > int(maxIdentifierLen) {
			return fmt.Errorf("subscribe identifier len %d is greater than %d", len(pld.Identifier), maxIdentifierLen)
		}

		maxMetaLen := config.MaxSubscribeMetaLen.GetValueAtHeight(height)
		if len(pld.Meta) > int(maxMetaLen) {
			return fmt.Errorf("subscribe meta len %d is greater than %d", len(pld.Meta), maxMetaLen)
		}
	case pb.UNSUBSCRIBE_TYPE:
		pld := payload.(*pb.Unsubscribe)

		if err := verifyPubSubTopic(pld.Topic, height); err != nil {
			return err
		}
	case pb.GENERATE_ID_TYPE:
		pld := payload.(*pb.GenerateID)
		err := crypto.CheckPublicKey(pld.PublicKey)
		if err != nil {
			return fmt.Errorf("decode pubkey error: %v", err)
		}

		if err = CheckAmount(pld.RegistrationFee); err != nil {
			return err
		}

		if common.Fixed64(pld.RegistrationFee) < common.Fixed64(config.MinGenIDRegistrationFee) {
			return errors.New("registration fee is lower than MinGenIDRegistrationFee")
		}

		txnHash := txn.Hash()
		if txnHash.CompareTo(config.MaxGenerateIDTxnHash.GetValueAtHeight(height)) > 0 {
			return errors.New("txn hash is greater than MaxGenerateIDTxnHash")
		}
	case pb.NANO_PAY_TYPE:
		pld := payload.(*pb.NanoPay)

		if len(pld.Sender) != common.UINT160SIZE && len(pld.Recipient) != common.UINT160SIZE {
			return errors.New("length of programhash error")
		}

		donationProgramhash, _ := common.ToScriptHash(config.DonationAddress)
		if bytes.Equal(pld.Sender, donationProgramhash[:]) {
			return errors.New("illegal transaction sender")
		}

		if err = CheckAmount(pld.Amount); err != nil {
			return err
		}

		if pld.TxnExpiration > pld.NanoPayExpiration {
			return errors.New("txn expiration should be no later than nano pay expiration")
		}

	case pb.ISSUE_ASSET_TYPE:
		pld := payload.(*pb.IssueAsset)
		if len(pld.Sender) != common.UINT160SIZE {
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
		return fmt.Errorf("invalid transaction payload type %v", txn.UnsignedTx.Payload.Type)
	}
	return nil
}

// VerifyTransactionWithLedger verifys a transaction with history transaction in ledger
func VerifyTransactionWithLedger(txn *transaction.Transaction, height uint32) error {
	if DefaultLedger.Store.IsDoubleSpend(txn) {
		return errors.New("[VerifyTransactionWithLedger] IsDoubleSpend check faild")
	}

	if DefaultLedger.Store.IsTxHashDuplicate(txn.Hash()) {
		return errors.New("[VerifyTransactionWithLedger] duplicate transaction check faild")
	}

	payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return errors.New("unpack transactiion's payload error")
	}

	pg, err := txn.GetProgramHashes()
	if err != nil {
		return err
	}

	switch txn.UnsignedTx.Payload.Type {
	case pb.NANO_PAY_TYPE:
	case pb.SIG_CHAIN_TXN_TYPE:
	default:
		if txn.UnsignedTx.Nonce < DefaultLedger.Store.GetNonce(pg[0]) {
			return errors.New("nonce is too low")
		}
	}

	var amount int64

	switch txn.UnsignedTx.Payload.Type {
	case pb.COINBASE_TYPE:
		donationAmount, err := DefaultLedger.Store.GetDonation()
		if err != nil {
			return err
		}

		donationProgramhash, _ := common.ToScriptHash(config.DonationAddress)
		amount := DefaultLedger.Store.GetBalance(donationProgramhash)
		if amount < donationAmount {
			return errors.New("not sufficient funds in doation account")
		}
	case pb.TRANSFER_ASSET_TYPE:
		pld := payload.(*pb.TransferAsset)
		amount += pld.Amount
	case pb.SIG_CHAIN_TXN_TYPE:
	case pb.REGISTER_NAME_TYPE:
		pld := payload.(*pb.RegisterName)
		if config.LegacyNameService.GetValueAtHeight(height) {
			name, err := DefaultLedger.Store.GetName_legacy(pld.Registrant)
			if name != "" {
				return fmt.Errorf("pubKey %s already has registered name %s", hex.EncodeToString(pld.Registrant), name)
			}
			if err != nil {
				return err
			}

			registrant, err := DefaultLedger.Store.GetRegistrant_legacy(pld.Name)
			if err != nil {
				return err
			}
			if registrant != nil {
				return fmt.Errorf("name %s is already registered for pubKey %+v", pld.Name, registrant)
			}
		} else {
			registrant, _, err := DefaultLedger.Store.GetRegistrant(pld.Name)
			if err != nil {
				return err
			}
			if len(registrant) > 0 && !bytes.Equal(registrant, pld.Registrant) {
				return fmt.Errorf("name %s is already registered for pubKey %+v", pld.Name, registrant)
			}
			amount += pld.RegistrationFee
		}
	case pb.TRANSFER_NAME_TYPE:
		pld := payload.(*pb.TransferName)

		registrant, _, err := DefaultLedger.Store.GetRegistrant(pld.Name)
		if err != nil {
			return err
		}
		if len(registrant) == 0 {
			return fmt.Errorf("can not transfer unregistered name")
		}
		if bytes.Equal(registrant, pld.Recipient) {
			return fmt.Errorf("can not transfer names to its owner")
		}
		if !bytes.Equal(registrant, pld.Registrant) {
			return fmt.Errorf("registrant incorrect")
		}
		senderPubkey, err := program.GetPublicKeyFromCode(txn.Programs[0].Code)
		if err != nil {
			return err
		}
		if !bytes.Equal(registrant, senderPubkey) {
			return fmt.Errorf("can not transfer names which did not belongs to you")
		}

	case pb.DELETE_NAME_TYPE:
		pld := payload.(*pb.DeleteName)
		if config.LegacyNameService.GetValueAtHeight(height) {
			name, err := DefaultLedger.Store.GetName_legacy(pld.Registrant)
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("no name registered for pubKey %+v", pld.Registrant)
			} else if name != pld.Name {
				return fmt.Errorf("no name %s registered for pubKey %+v", pld.Name, pld.Registrant)
			}
		} else {
			registrant, _, err := DefaultLedger.Store.GetRegistrant(pld.Name)
			if err != nil {
				return err
			}
			if len(registrant) == 0 {
				return fmt.Errorf("name doesn't exist")
			}
			if !bytes.Equal(registrant, pld.Registrant) {
				return fmt.Errorf("can not delete name which did not belongs to you")
			}
		}

	case pb.SUBSCRIBE_TYPE:
		pld := payload.(*pb.Subscribe)
		subscribed, err := DefaultLedger.Store.IsSubscribed(pld.Topic, pld.Bucket, pld.Subscriber, pld.Identifier)
		if err != nil {
			return err
		}
		if !subscribed {
			subscriptionCount := DefaultLedger.Store.GetSubscribersCount(pld.Topic, pld.Bucket)
			maxSubscriptionCount := config.MaxSubscriptionsCount
			if subscriptionCount >= maxSubscriptionCount {
				return fmt.Errorf("subscription count to %s can't be more than %d", pld.Topic, maxSubscriptionCount)
			}
		}
	case pb.UNSUBSCRIBE_TYPE:
		pld := payload.(*pb.Unsubscribe)
		subscribed, err := DefaultLedger.Store.IsSubscribed(pld.Topic, 0, pld.Subscriber, pld.Identifier)
		if err != nil {
			return err
		}
		if !subscribed {
			return fmt.Errorf("subscription to %s doesn't exist", pld.Topic)
		}
	case pb.GENERATE_ID_TYPE:
		pld := payload.(*pb.GenerateID)
		id, err := DefaultLedger.Store.GetID(pld.PublicKey)
		if err != nil {
			return err
		}
		if len(id) != 0 {
			return ErrIDRegistered
		}
		amount += pld.RegistrationFee
	case pb.NANO_PAY_TYPE:
		pld := payload.(*pb.NanoPay)

		channelBalance, _, err := DefaultLedger.Store.GetNanoPay(
			common.BytesToUint160(pld.Sender),
			common.BytesToUint160(pld.Recipient),
			pld.Id,
		)
		if err != nil {
			return err
		}

		if height > pld.TxnExpiration {
			return errors.New("nano pay txn has expired")
		}
		if height > pld.NanoPayExpiration {
			return errors.New("nano pay has expired")
		}

		balanceToClaim := pld.Amount - int64(channelBalance)
		if balanceToClaim <= 0 {
			return errors.New("invalid amount")
		}
		amount += balanceToClaim
	case pb.ISSUE_ASSET_TYPE:
		assetID := txn.Hash()
		_, _, _, _, err := DefaultLedger.Store.GetAsset(assetID)
		if err == nil {
			return ErrDuplicateIssueAssetTxn
		}
	default:
		return fmt.Errorf("invalid transaction payload type %v", txn.UnsignedTx.Payload.Type)
	}

	balance := DefaultLedger.Store.GetBalance(pg[0])
	if int64(balance) < amount+txn.UnsignedTx.Fee {
		return errors.New("not sufficient funds")
	}

	return nil
}

type subscription struct {
	topic      string
	bucket     uint32
	subscriber string
	identifier string
}

type subscriptionInfo struct {
	new  bool
	meta string
}

type nanoPay struct {
	sender    string
	recipient string
	nonce     uint64
}

type BlockValidationState struct {
	sync.RWMutex
	txnlist                 map[common.Uint256]struct{}
	totalAmount             map[common.Uint160]common.Fixed64
	registeredNames         map[string]struct{}
	nameRegistrants         map[string]struct{}
	generateIDs             map[string]struct{}
	subscriptions           map[subscription]subscriptionInfo
	subscriptionCount       map[string]int
	subscriptionCountChange map[string]int
	nanoPays                map[nanoPay]struct{}

	changes []func()
}

func NewBlockValidationState() *BlockValidationState {
	bvs := &BlockValidationState{}
	bvs.initBlockValidationState()
	return bvs
}

func (bvs *BlockValidationState) initBlockValidationState() {
	bvs.txnlist = make(map[common.Uint256]struct{}, 0)
	bvs.totalAmount = make(map[common.Uint160]common.Fixed64, 0)
	bvs.registeredNames = make(map[string]struct{}, 0)
	bvs.nameRegistrants = make(map[string]struct{}, 0)
	bvs.generateIDs = make(map[string]struct{}, 0)
	bvs.subscriptions = make(map[subscription]subscriptionInfo, 0)
	bvs.subscriptionCount = make(map[string]int, 0)
	bvs.subscriptionCountChange = make(map[string]int, 0)
	bvs.nanoPays = make(map[nanoPay]struct{}, 0)
}

func (bvs *BlockValidationState) Close() {
	bvs.txnlist = nil
	bvs.totalAmount = nil
	bvs.registeredNames = nil
	bvs.nameRegistrants = nil
	bvs.generateIDs = nil
	bvs.subscriptions = nil
	bvs.subscriptionCount = nil
	bvs.nanoPays = nil
}

func (bvs *BlockValidationState) addChange(change func()) {
	bvs.changes = append(bvs.changes, change)
}

func (bvs *BlockValidationState) Commit() {
	for _, change := range bvs.changes {
		change()
	}
	for topic, change := range bvs.subscriptionCountChange {
		bvs.subscriptionCount[topic] += change
	}
	bvs.Reset()
}

func (bvs *BlockValidationState) Reset() {
	bvs.changes = nil
	bvs.subscriptionCountChange = make(map[string]int, 0)
}

func (bvs *BlockValidationState) GetSubscribers(topic string) []string {
	subscribers := make([]string, 0)
	for key, info := range bvs.subscriptions {
		if key.topic == topic && info.new {
			subscriber := address.MakeAddressString([]byte(key.subscriber), key.identifier)
			subscribers = append(subscribers, subscriber)
		}
	}
	return subscribers
}

func (bvs *BlockValidationState) GetSubscribersWithMeta(topic string) map[string]string {
	subscribers := make(map[string]string)
	for key, info := range bvs.subscriptions {
		if key.topic == topic && info.new {
			subscriber := address.MakeAddressString([]byte(key.subscriber), key.identifier)
			subscribers[subscriber] = info.meta
		}
	}
	return subscribers
}

// VerifyTransactionWithBlock verifies a transaction with current transaction pool in memory
func (bvs *BlockValidationState) VerifyTransactionWithBlock(txn *transaction.Transaction, height uint32) (e error) {
	//1.check weather have duplicate transaction.
	if _, exist := bvs.txnlist[txn.Hash()]; exist {
		return errors.New("[VerifyTransactionWithBlock] duplicate transaction exist in block")
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
		return errors.New("[VerifyTransactionWithBlock] payload unpack error")
	}

	pg, err := txn.GetProgramHashes()
	if err != nil {
		return err
	}
	sender := pg[0]
	var amount common.Fixed64
	fee := common.Fixed64(txn.UnsignedTx.Fee)

	switch txn.UnsignedTx.Payload.Type {
	case pb.COINBASE_TYPE:
		coinbase := payload.(*pb.Coinbase)
		donationAmount, err := DefaultLedger.Store.GetDonation()
		if err != nil {
			return err
		}
		if common.Fixed64(coinbase.Amount) != GetRewardByHeight(height)+donationAmount {
			return errors.New("mining reward incorrectly")
		}
	case pb.TRANSFER_ASSET_TYPE:
		transfer := payload.(*pb.TransferAsset)
		amount = common.Fixed64(transfer.Amount)
	case pb.REGISTER_NAME_TYPE:
		namePayload := payload.(*pb.RegisterName)

		name := namePayload.Name
		if _, ok := bvs.registeredNames[name]; ok {
			return errors.New("[VerifyTransactionWithBlock] duplicate name exist in block")
		}

		registrant := hex.EncodeToString(namePayload.Registrant)
		if config.LegacyNameService.GetValueAtHeight(height) {
			if _, ok := bvs.nameRegistrants[registrant]; ok {
				return errors.New("[VerifyTransactionWithBlock] duplicate registrant exist in block")
			}
		}
		amount = common.Fixed64(namePayload.RegistrationFee)

		defer func() {
			if e == nil {
				bvs.addChange(func() {
					bvs.registeredNames[name] = struct{}{}
					bvs.nameRegistrants[registrant] = struct{}{}
				})
			}
		}()
	case pb.TRANSFER_NAME_TYPE:
		namePayload := payload.(*pb.TransferName)
		name := namePayload.Name

		defer func() {
			if e == nil {
				bvs.addChange(func() {
					bvs.registeredNames[name] = struct{}{}
				})
			}
		}()
	case pb.DELETE_NAME_TYPE:
		namePayload := payload.(*pb.DeleteName)

		name := namePayload.Name
		if _, ok := bvs.registeredNames[name]; ok {
			return errors.New("[VerifyTransactionWithBlock] duplicate name exist in block")
		}

		registrant := hex.EncodeToString(namePayload.Registrant)
		if config.LegacyNameService.GetValueAtHeight(height) {
			if _, ok := bvs.nameRegistrants[registrant]; ok {
				return errors.New("[VerifyTransactionWithBlock] duplicate registrant exist in block")
			}
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
		key := subscription{topic, bucket, string(subscribePayload.Subscriber), subscribePayload.Identifier}
		if _, ok := bvs.subscriptions[key]; ok {
			return errors.New("[VerifyTransactionWithBlock] duplicate subscription exist in block")
		}

		subscribed, err := DefaultLedger.Store.IsSubscribed(subscribePayload.Topic, bucket, subscribePayload.Subscriber, subscribePayload.Identifier)
		if err != nil {
			return err
		}
		subscriptionCount := bvs.subscriptionCount[topic]
		subscriptionCountChange := bvs.subscriptionCountChange[topic]
		if !subscribed {
			ledgerSubscriptionCount := DefaultLedger.Store.GetSubscribersCount(topic, bucket)
			if ledgerSubscriptionCount+subscriptionCount+subscriptionCountChange >= config.MaxSubscriptionsCount {
				return errors.New("[VerifyTransactionWithBlock] subscription limit exceeded in block")
			}
			bvs.subscriptionCountChange[topic]++
		}

		defer func() {
			if e == nil {
				bvs.addChange(func() {
					bvs.subscriptions[key] = subscriptionInfo{new: !subscribed, meta: subscribePayload.Meta}
				})
			}
		}()
	case pb.UNSUBSCRIBE_TYPE:
		unsubscribePayload := payload.(*pb.Unsubscribe)
		topic := unsubscribePayload.Topic
		key := subscription{topic, 0, string(unsubscribePayload.Subscriber), unsubscribePayload.Identifier}
		if _, ok := bvs.subscriptions[key]; ok {
			return errors.New("[VerifyTransactionWithBlock] duplicate subscription exist in block")
		}

		subscribed, err := DefaultLedger.Store.IsSubscribed(unsubscribePayload.Topic, 0, unsubscribePayload.Subscriber, unsubscribePayload.Identifier)
		if err != nil {
			return err
		}
		if !subscribed {
			return errors.New("[VerifyTransactionWithBlock] subscription doesn't exist")
		}
		bvs.subscriptionCountChange[topic]--

		defer func() {
			if e == nil {
				bvs.addChange(func() {
					bvs.subscriptions[key] = subscriptionInfo{new: false}
				})
			}
		}()
	case pb.GENERATE_ID_TYPE:
		generateIDPayload := payload.(*pb.GenerateID)
		amount = common.Fixed64(generateIDPayload.RegistrationFee)
		publicKey := hex.EncodeToString(generateIDPayload.PublicKey)
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
			return errors.New("[VerifyTransactionWithBlock] nano pay txn has expired")
		}
		if height > npPayload.NanoPayExpiration {
			return errors.New("[VerifyTransactionWithBlock] nano pay has expired")
		}
		key := nanoPay{hex.EncodeToString(npPayload.Sender), hex.EncodeToString(npPayload.Recipient), npPayload.Id}
		if _, ok := bvs.nanoPays[key]; ok {
			return errors.New("[VerifyTransactionWithBlock] duplicate payment channel exist in block")
		}

		channelBalance, _, err := DefaultLedger.Store.GetNanoPay(
			common.BytesToUint160(npPayload.Sender),
			common.BytesToUint160(npPayload.Recipient),
			npPayload.Id,
		)
		if err != nil {
			return err
		}
		amount = common.Fixed64(npPayload.Amount) - channelBalance

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
			return errors.New("[VerifyTransactionWithBlock] not sufficient funds")
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
			return errors.New("[CleanSubmittedTransactions] payload unpack error")
		}

		pg, err := txn.GetProgramHashes()
		if err != nil {
			return err
		}
		sender := pg[0]
		var amount common.Fixed64
		fee := common.Fixed64(txn.UnsignedTx.Fee)

		switch txn.UnsignedTx.Payload.Type {
		case pb.TRANSFER_ASSET_TYPE:
			transfer := payload.(*pb.TransferAsset)
			amount = common.Fixed64(transfer.Amount)
		case pb.REGISTER_NAME_TYPE:
			namePayload := payload.(*pb.RegisterName)
			amount = common.Fixed64(namePayload.RegistrationFee)

			name := namePayload.Name
			delete(bvs.registeredNames, name)

			registrant := hex.EncodeToString(namePayload.Registrant)
			delete(bvs.nameRegistrants, registrant)

		case pb.TRANSFER_NAME_TYPE:
			namePayload := payload.(*pb.TransferName)
			name := namePayload.Name
			delete(bvs.registeredNames, name)

		case pb.DELETE_NAME_TYPE:
			namePayload := payload.(*pb.DeleteName)

			name := namePayload.Name
			delete(bvs.registeredNames, name)

			registrant := hex.EncodeToString(namePayload.Registrant)
			delete(bvs.nameRegistrants, registrant)
		case pb.SUBSCRIBE_TYPE:
			subscribePayload := payload.(*pb.Subscribe)
			topic := subscribePayload.Topic
			bucket := subscribePayload.Bucket
			key := subscription{topic, bucket, string(subscribePayload.Subscriber), subscribePayload.Identifier}
			if info, ok := bvs.subscriptions[key]; ok {
				delete(bvs.subscriptions, key)

				if info.new {
					bvs.subscriptionCount[topic]--

					if bvs.subscriptionCount[topic] == 0 {
						delete(bvs.subscriptionCount, topic)
					}
				}
			}
		case pb.UNSUBSCRIBE_TYPE:
			unsubscribePayload := payload.(*pb.Unsubscribe)
			topic := unsubscribePayload.Topic
			key := subscription{topic, 0, string(unsubscribePayload.Subscriber), unsubscribePayload.Identifier}
			if _, ok := bvs.subscriptions[key]; ok {
				delete(bvs.subscriptions, key)

				bvs.subscriptionCount[topic]++
			}
		case pb.GENERATE_ID_TYPE:
			generateIdPayload := payload.(*pb.GenerateID)
			amount = common.Fixed64(generateIdPayload.RegistrationFee)
			publicKey := hex.EncodeToString(generateIdPayload.PublicKey)
			delete(bvs.generateIDs, publicKey)
		case pb.NANO_PAY_TYPE:
			npPayload := payload.(*pb.NanoPay)
			key := nanoPay{hex.EncodeToString(npPayload.Sender), hex.EncodeToString(npPayload.Recipient), npPayload.Id}
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
				return errors.New("[CleanSubmittedTransactions] inconsistent block validation state")
			}
		}
	}

	return nil
}

func (bvs *BlockValidationState) RefreshBlockValidationState(txns []*transaction.Transaction) map[common.Uint256]error {
	bvs.initBlockValidationState()
	errMap := make(map[common.Uint256]error, 0)
	for _, tx := range txns {
		if err := bvs.VerifyTransactionWithBlock(tx, 0); err != nil {
			errMap[tx.Hash()] = err
		}
	}
	bvs.Commit()
	return errMap
}
