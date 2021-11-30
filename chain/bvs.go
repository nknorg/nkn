package chain

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/nknorg/nkn/v2/common"

	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/transaction"
	"github.com/nknorg/nkn/v2/util/address"
)

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
	nanoPayAmount           map[common.Uint160]common.Fixed64
	nanoPayPayloads         map[common.Uint256]*pb.NanoPay

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
	bvs.nanoPayAmount = make(map[common.Uint160]common.Fixed64, 0)
	bvs.nanoPayPayloads = make(map[common.Uint256]*pb.NanoPay, 0)
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
	bvs.nanoPayAmount = nil
	bvs.nanoPayPayloads = nil
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

	payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return errors.New("[VerifyTransactionWithBlock] payload unpack error")
	}

	pg, err := txn.GetProgramHashes()
	if err != nil {
		return err
	}
	sender := pg[0]

	var amount, npAmount common.Fixed64
	fee := common.Fixed64(txn.UnsignedTx.Fee)

	switch txn.UnsignedTx.Payload.Type {
	case pb.PayloadType_COINBASE_TYPE:
		coinbase := payload.(*pb.Coinbase)
		donationAmount, err := DefaultLedger.Store.GetDonation()
		if err != nil {
			return err
		}
		if common.Fixed64(coinbase.Amount) != GetRewardByHeight(height)+donationAmount {
			return errors.New("mining reward incorrectly")
		}
	case pb.PayloadType_TRANSFER_ASSET_TYPE:
		transfer := payload.(*pb.TransferAsset)
		amount = common.Fixed64(transfer.Amount)
	case pb.PayloadType_REGISTER_NAME_TYPE:
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
	case pb.PayloadType_TRANSFER_NAME_TYPE:
		namePayload := payload.(*pb.TransferName)
		name := namePayload.Name

		defer func() {
			if e == nil {
				bvs.addChange(func() {
					bvs.registeredNames[name] = struct{}{}
				})
			}
		}()
	case pb.PayloadType_DELETE_NAME_TYPE:
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
	case pb.PayloadType_SUBSCRIBE_TYPE:
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
			if config.MaxSubscriptionsCount > 0 {
				ledgerSubscriptionCount, err := DefaultLedger.Store.GetSubscribersCount(topic, bucket, nil, context.Background())
				if err != nil {
					return err
				}
				if ledgerSubscriptionCount+subscriptionCount+subscriptionCountChange >= config.MaxSubscriptionsCount {
					return errors.New("[VerifyTransactionWithBlock] subscription limit exceeded in block")
				}
			}
			bvs.subscriptionCountChange[topic]++
		}

		defer func() {
			if e == nil {
				bvs.addChange(func() {
					bvs.subscriptions[key] = subscriptionInfo{new: !subscribed, meta: string(subscribePayload.Meta)}
				})
			}
		}()
	case pb.PayloadType_UNSUBSCRIBE_TYPE:
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
	case pb.PayloadType_GENERATE_ID_TYPE:
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
	case pb.PayloadType_NANO_PAY_TYPE:
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

		if common.Fixed64(npPayload.Amount) <= channelBalance {
			return errors.New("invalid nanopay amount")
		}

		npAmount = common.Fixed64(npPayload.Amount) - channelBalance

		defer func() {
			if e == nil {
				bvs.addChange(func() {
					bvs.nanoPays[key] = struct{}{}
					bvs.nanoPayPayloads[txn.Hash()] = npPayload
				})
			}
		}()

	case pb.PayloadType_ISSUE_ASSET_TYPE:
	}

	if amount+npAmount+fee > 0 {
		balance := DefaultLedger.Store.GetBalance(sender)
		if balance < bvs.totalAmount[sender]+bvs.nanoPayAmount[sender]+amount+npAmount+fee {
			return errors.New("not sufficient funds")
		}

		defer func() {
			if e == nil {
				bvs.addChange(func() {
					bvs.totalAmount[sender] += amount + fee
					bvs.nanoPayAmount[sender] += npAmount
				})
			}
		}()
	}

	return nil
}

func (bvs *BlockValidationState) CleanSubmittedTransactions(txns []*transaction.Transaction) error {
	for _, txn := range txns {
		if _, exist := bvs.txnlist[txn.Hash()]; !exist {
			continue
		}

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
		case pb.PayloadType_TRANSFER_ASSET_TYPE:
			transfer := payload.(*pb.TransferAsset)
			amount = common.Fixed64(transfer.Amount)
		case pb.PayloadType_REGISTER_NAME_TYPE:
			namePayload := payload.(*pb.RegisterName)
			amount = common.Fixed64(namePayload.RegistrationFee)

			name := namePayload.Name
			delete(bvs.registeredNames, name)

			registrant := hex.EncodeToString(namePayload.Registrant)
			delete(bvs.nameRegistrants, registrant)
		case pb.PayloadType_TRANSFER_NAME_TYPE:
			namePayload := payload.(*pb.TransferName)
			name := namePayload.Name
			delete(bvs.registeredNames, name)
		case pb.PayloadType_DELETE_NAME_TYPE:
			namePayload := payload.(*pb.DeleteName)

			name := namePayload.Name
			delete(bvs.registeredNames, name)

			registrant := hex.EncodeToString(namePayload.Registrant)
			delete(bvs.nameRegistrants, registrant)
		case pb.PayloadType_SUBSCRIBE_TYPE:
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
		case pb.PayloadType_UNSUBSCRIBE_TYPE:
			unsubscribePayload := payload.(*pb.Unsubscribe)
			topic := unsubscribePayload.Topic
			key := subscription{topic, 0, string(unsubscribePayload.Subscriber), unsubscribePayload.Identifier}
			if _, ok := bvs.subscriptions[key]; ok {
				delete(bvs.subscriptions, key)

				bvs.subscriptionCount[topic]++
			}
		case pb.PayloadType_GENERATE_ID_TYPE:
			generateIDPayload := payload.(*pb.GenerateID)
			amount = common.Fixed64(generateIDPayload.RegistrationFee)
			publicKey := hex.EncodeToString(generateIDPayload.PublicKey)
			delete(bvs.generateIDs, publicKey)
		case pb.PayloadType_NANO_PAY_TYPE:
			npPayload := payload.(*pb.NanoPay)
			key := nanoPay{hex.EncodeToString(npPayload.Sender), hex.EncodeToString(npPayload.Recipient), npPayload.Id}
			delete(bvs.nanoPays, key)
			delete(bvs.nanoPayPayloads, txn.Hash())
		case pb.PayloadType_ISSUE_ASSET_TYPE:
		}

		if amount > 0 || fee > 0 {
			if _, ok := bvs.totalAmount[sender]; ok && bvs.totalAmount[sender] >= amount+fee {
				bvs.totalAmount[sender] -= amount + fee
				if bvs.totalAmount[sender] == 0 {
					delete(bvs.totalAmount, sender)
				}
			} else {
				return errors.New("inconsistent block validation state")
			}
		}
	}

	if len(bvs.nanoPayAmount) > 0 {
		bvs.nanoPayAmount = make(map[common.Uint160]common.Fixed64, len(bvs.nanoPayAmount))
		for _, npPayload := range bvs.nanoPayPayloads {
			channelBalance, _, err := DefaultLedger.Store.GetNanoPay(
				common.BytesToUint160(npPayload.Sender),
				common.BytesToUint160(npPayload.Recipient),
				npPayload.Id,
			)
			if err != nil {
				return err
			}

			if common.Fixed64(npPayload.Amount) <= channelBalance {
				return errors.New("inconsistent block validation state")
			}

			bvs.nanoPayAmount[common.BytesToUint160(npPayload.Sender)] += common.Fixed64(npPayload.Amount) - channelBalance
		}
	}

	return nil
}

func (bvs *BlockValidationState) RefreshBlockValidationState(txns []*transaction.Transaction) map[common.Uint256]error {
	bvs.initBlockValidationState()
	errMap := make(map[common.Uint256]error, 0)
	for _, tx := range txns {
		if err := bvs.VerifyTransactionWithBlock(tx, DefaultLedger.Store.GetHeight()+1); err != nil {
			errMap[tx.Hash()] = err
		}
	}
	bvs.Commit()
	return errMap
}
