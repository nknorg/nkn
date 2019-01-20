package transaction

import (
	"errors"
	"fmt"
	"math"
	"regexp"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/asset"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/core/validation"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"

	"github.com/syndtr/goleveldb/leveldb"
)

const (
	SubscriptionsLimit = 1000
	BucketsLimit = 1000
	MaxSubscriptionDuration = 65535
)

type TxnStore interface {
	GetTransaction(hash Uint256) (*Transaction, error)
	GetQuantityIssued(AssetId Uint256) (Fixed64, error)
	IsDoubleSpend(tx *Transaction) bool
	GetAsset(hash Uint256) (*asset.Asset, error)
	GetPrepaidInfo(programHash Uint160) (*Fixed64, *Fixed64, error)
	IsTxHashDuplicate(txhash Uint256) bool
	GetName(registrant []byte) (*string, error)
	GetRegistrant(name string) ([]byte, error)
	IsSubscribed(subscriber []byte, identifier string, topic string, bucket uint32) (bool, error)
	GetSubscribersCount(topic string, bucket uint32) int
}

type Iterator interface {
	Iterate(handler func(item *Transaction) ErrCode) ErrCode
}

// VerifyTransaction verifys received single transaction
func VerifyTransaction(Tx *Transaction) ErrCode {
	if CheckInputOutputTooLong(Tx) {
		return ErrInputOutputTooLong
	}
	if err := CheckDuplicateInput(Tx); err != nil {
		log.Warning("[VerifyTransaction],", err)
		return ErrDuplicateInput
	}

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
		if err, ok := err.(ErrCode); ok {
			return err
		}
		return ErrTransactionPayload
	}

	return ErrNoError
}

// VerifyTransactionWithBlock verifys a transaction with current transaction pool in memory
func VerifyTransactionWithBlock(iterator Iterator) ErrCode {
	//initial
	txnlist := make(map[Uint256]struct{}, 0)
	txPoolInputs := make(map[string]struct{}, 0)
	issueSummary := make(map[Uint256]Fixed64, 0)
	registeredNames := make(map[string]struct{}, 0)
	nameRegistrants := make(map[string]struct{}, 0)

	type subscription struct {topic string; bucket uint32; subscriber string}
	type topicKey struct {topic string; bucket uint32}
	subscriptions := make(map[subscription]struct{}, 0)

	subscriptionCount := make(map[topicKey]int, 0)

	//start check
	return iterator.Iterate(func(txn *Transaction) ErrCode {
		//1.check weather have duplicate transaction.
		if _, exist := txnlist[txn.Hash()]; exist {
			log.Warning("[VerifyTransactionWithBlock], duplicate transaction exist in block.")
			return ErrDuplicatedTx
		} else {
			txnlist[txn.Hash()] = struct{}{}
		}
		//2.check Duplicate Utxo input
		for _, UTXOinput := range txn.Inputs {
			inputString := UTXOinput.ToString()
			if _, ok := txPoolInputs[inputString]; ok {
				log.Warning("[VerifyTransactionWithBlock], duplicate input exist in block.")
				return ErrDuplicateInput
			}
			txPoolInputs[inputString] = struct{}{}
		}
		//3.check issue amount
		switch txn.TxType {
		case Coinbase:
			if txn.Outputs[0].Value != Fixed64(config.DefaultMiningReward*StorageFactor) {
				log.Warning("Mining reward incorrectly.")
				return ErrMineReward
			}
		case IssueAsset:
			results := txn.GetMergedAssetIDValueFromOutputs()
			for k, delta := range results {
				issueSummary[k] = issueSummary[k] + delta

				//Get the Asset amount when RegisterAsseted.
				trx, err := Store.GetTransaction(k)
				if trx.TxType != RegisterAsset {
					log.Warning("[VerifyTransactionWithBlock], TxType is illegal.")
					return ErrSummaryAsset
				}
				AssetReg := trx.Payload.(*payload.RegisterAsset)

				//Get the amount has been issued of this assetID
				var quantity_issued Fixed64
				if AssetReg.Amount < Fixed64(0) {
					continue
				} else {
					quantity_issued, err = Store.GetQuantityIssued(k)
					if err != nil {
						log.Warning("[VerifyTransactionWithBlock], GetQuantityIssued failed.")
						return ErrSummaryAsset
					}
				}

				//calc weather out off the amount when Registed.
				//AssetReg.Amount : amount when RegisterAsset of this assedID
				//quantity_issued : amount has been issued of this assedID
				//issueSummary[k] : amount in transactionPool of this assedID of issue transaction.
				if AssetReg.Amount-quantity_issued < issueSummary[k] {
					log.Warning("[VerifyTransactionWithBlock], Amount check error.")
					return ErrSummaryAsset
				}
			}
		case RegisterName:
			namePayload := txn.Payload.(*payload.RegisterName)

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
		case DeleteName:
			namePayload := txn.Payload.(*payload.DeleteName)

			if txn.PayloadVersion > 0 {
				name := namePayload.Name
				if _, ok := registeredNames[name]; ok {
					log.Warning("[VerifyTransactionWithBlock], duplicate name exist in block.")
					return ErrDuplicateName
				}
				registeredNames[name] = struct{}{}
			}

			registrant := BytesToHexString(namePayload.Registrant)
			if _, ok := nameRegistrants[registrant]; ok {
				log.Warning("[VerifyTransactionWithBlock], duplicate registrant exist in block.")
				return ErrDuplicateName
			}
			nameRegistrants[registrant] = struct{}{}
		case Subscribe:
			subscribePayload := txn.Payload.(*payload.Subscribe)
			topic := subscribePayload.Topic
			bucket := subscribePayload.Bucket
			key := subscription{topic, bucket, subscribePayload.SubscriberString()}
			if _, ok := subscriptions[key]; ok {
				log.Warning("[VerifyTransactionWithBlock], duplicate subscription exist in block.")
				return ErrDuplicateSubscription
			}
			subscriptions[key] = struct{}{}

			topicKey := topicKey{topic, bucket}
			if _, ok := subscriptionCount[topicKey]; !ok {
				subscriptionCount[topicKey] = Store.GetSubscribersCount(topic, bucket)
			}
			if subscriptionCount[topicKey] >= SubscriptionsLimit {
				log.Warning("[VerifyTransactionWithBlock], subscription limit exceeded in block.")
				return ErrSubscriptionLimit
			}
			subscriptionCount[topicKey]++
		}

		return ErrNoError
	})
}

// VerifyTransactionWithLedger verifys a transaction with history transaction in ledger
func VerifyTransactionWithLedger(Tx *Transaction) ErrCode {
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

// CheckInputOutputLen validate len(inputs/outputs) less than MAX of ReferTxOutputIndex
func CheckInputOutputTooLong(tx *Transaction) bool {
	return len(tx.Inputs) > MaxRefIndex || len(tx.Outputs) > MaxRefIndex
}

//validate the transaction of duplicate UTXO input
func CheckDuplicateInput(tx *Transaction) error {
	dupMap := make(map[string]struct{})
	for _, utxoin := range tx.Inputs {
		k := utxoin.ToString()
		if _, ok := dupMap[k]; ok { // ok means duplicate
			return errors.New("invalid transaction")
		}
		dupMap[k] = struct{}{}
	}
	return nil
}

func IsDoubleSpend(tx *Transaction) bool {
	return Store.IsDoubleSpend(tx)
}

func CheckAssetPrecision(Tx *Transaction) error {
	if len(Tx.Outputs) == 0 {
		return nil
	}
	assetOutputs := make(map[Uint256][]*TxnOutput, len(Tx.Outputs))

	for _, v := range Tx.Outputs {
		assetOutputs[v.AssetID] = append(assetOutputs[v.AssetID], v)
	}
	for k, outputs := range assetOutputs {
		asset, err := Store.GetAsset(k)
		if err != nil {
			return errors.New("The asset not exist in local blockchain.")
		}
		precision := asset.Precision
		for _, output := range outputs {
			if checkAmountPrecise(output.Value, precision) {
				return errors.New("The precision of asset is incorrect.")
			}
		}
	}
	return nil
}

func CheckTransactionBalance(txn *Transaction) error {
	if txn.TxType == Coinbase || txn.TxType == Prepaid ||
		txn.TxType == Withdraw || txn.TxType == Commit {
		return nil
	}
	for _, v := range txn.Outputs {
		if v.Value <= Fixed64(0) {
			return errors.New("invalid transaction UTXO output.")
		}
	}
	if txn.TxType == IssueAsset {
		if len(txn.Inputs) > 0 {
			return errors.New("invalid Issue transaction.")
		}
		return nil
	}
	results, err := txn.GetTransactionResults()
	if err != nil {
		return err
	}
	for k, v := range results {
		if v != 0 {
			log.Debug(fmt.Sprintf("AssetID %x in Transfer transactions %x , Input/output UTXO not equal.", k, txn.Hash()))
			return errors.New(fmt.Sprintf("AssetID %x in Transfer transactions %x , Input/output UTXO not equal.", k, txn.Hash()))
		}
	}
	return nil
}

func CheckAttributeProgram(Tx *Transaction) error {
	//TODO: implement CheckAttributeProgram
	return nil
}

func CheckTransactionContracts(Tx *Transaction) error {
	flag, err := validation.VerifySignableData(Tx)
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

	switch pld := txn.Payload.(type) {
	case *payload.RegisterAsset:
		if pld.Asset.Precision < asset.MinPrecision || pld.Asset.Precision > asset.MaxPrecision {
			return errors.New("invalid asset Precision.")
		}
		if checkAmountPrecise(pld.Amount, pld.Asset.Precision) {
			return errors.New("invalid asset value,out of precise.")
		}
	case *payload.IssueAsset:
	case *payload.TransferAsset:
	case *payload.Coinbase:
	case *payload.Commit:
	case *payload.Prepaid:
		var inputAmount, outputAmount Fixed64
		for _, input := range txn.Inputs {
			reftxn, err := Store.GetTransaction(input.ReferTxID)
			if err != nil {
				return err
			}
			inputAmount += reftxn.Outputs[input.ReferTxOutputIndex].Value
		}
		for _, output := range txn.Outputs {
			outputAmount += output.Value
		}
		if inputAmount-outputAmount != pld.Amount {
			return errors.New("prepaid transaction balance unmatched")
		}
	case *payload.Withdraw:
		var outputAmount Fixed64

		for _, output := range txn.Outputs {
			outputAmount += output.Value
		}
		prepaidAmount, _, err := Store.GetPrepaidInfo(pld.ProgramHash)
		if err != nil {
			return err
		}
		if outputAmount > *prepaidAmount {
			return errors.New("asset is not enough")
		}
	case *payload.RegisterName:
		match, err := regexp.MatchString("([a-z]{8,12})", pld.Name)
		if err != nil {
			return err
		}
		if !match {
			return errors.New(fmt.Sprintf("name %s should only contain a-z and have length 8-12", pld.Name))
		}

		name, err := Store.GetName(pld.Registrant)
		if name != nil {
			return errors.New(fmt.Sprintf("pubKey %+v already has registered name %s", pld.Registrant, *name))
		}
		if err != leveldb.ErrNotFound {
			return err
		}

		registrant, err := Store.GetRegistrant(pld.Name)
		if registrant != nil {
			return errors.New(fmt.Sprintf("name %s is already registered for pubKey %+v", pld.Name, registrant))
		}
		if err != leveldb.ErrNotFound {
			return err
		}
	case *payload.DeleteName:
		name, err := Store.GetName(pld.Registrant)
		if err != leveldb.ErrNotFound {
			return err
		}
		if txn.PayloadVersion > 0 && *name != pld.Name {
			return errors.New(fmt.Sprintf("no name %s registered for pubKey %+v", pld.Name, pld.Registrant))
		} else if name == nil {
			return errors.New(fmt.Sprintf("no name registered for pubKey %+v", pld.Registrant))
		}
	case *payload.Subscribe:
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
			return errors.New(fmt.Sprintf("topic %s should only contain a-z and have length 8-12", topic))
		}

		subscribed, err := Store.IsSubscribed(pld.Subscriber, pld.Identifier, topic, bucket)
		if err != nil {
			return err
		}
		if subscribed {
			return ErrAlreadySubscribed
		}

		subscriptionCount := Store.GetSubscribersCount(topic, bucket)
		if subscriptionCount >= SubscriptionsLimit {
			return ErrSubscriptionLimit
		}
	default:
		return errors.New("[txValidator],invalidate transaction payload type.")
	}
	return nil
}
