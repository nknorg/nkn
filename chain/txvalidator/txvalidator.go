package txvalidator

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/crypto/ed25519"

	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/crypto"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/transaction"
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
	case pb.PayloadType_COINBASE_TYPE:
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
	case pb.PayloadType_TRANSFER_ASSET_TYPE:
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
	case pb.PayloadType_SIG_CHAIN_TXN_TYPE:
	case pb.PayloadType_REGISTER_NAME_TYPE:
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
	case pb.PayloadType_TRANSFER_NAME_TYPE:
		pld := payload.(*pb.TransferName)
		if len(pld.Registrant) != ed25519.PublicKeySize {
			return fmt.Errorf("registrant invalid")
		}
	case pb.PayloadType_DELETE_NAME_TYPE:
		pld := payload.(*pb.DeleteName)
		if len(pld.Registrant) != ed25519.PublicKeySize {
			return fmt.Errorf("registrant invalid")
		}
	case pb.PayloadType_SUBSCRIBE_TYPE:
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
	case pb.PayloadType_UNSUBSCRIBE_TYPE:
		pld := payload.(*pb.Unsubscribe)

		if err := verifyPubSubTopic(pld.Topic, height); err != nil {
			return err
		}
	case pb.PayloadType_GENERATE_ID_TYPE:
		pld := payload.(*pb.GenerateID)
		if pld.Version < config.AllowTxnGenerateIDMinVersion.GetValueAtHeight(height) || pld.Version > config.AllowTxnGenerateIDMaxVersion.GetValueAtHeight(height) {
			return fmt.Errorf("id version %d is not supported at height %d", pld.Version, height)
		}

		expectedSenderLength := 0
		if config.AllowGenerateIDSender.GetValueAtHeight(height) {
			expectedSenderLength = common.UINT160SIZE
		}
		if len(pld.Sender) != expectedSenderLength {
			return fmt.Errorf("program hash wrong size %d, expecting %d", len(pld.Sender), expectedSenderLength)
		}

		err := crypto.CheckPublicKey(pld.PublicKey)
		if err != nil {
			return fmt.Errorf("decode pubkey error: %v", err)
		}

		if err = CheckAmount(pld.RegistrationFee); err != nil {
			return err
		}

		if common.Fixed64(pld.RegistrationFee) < common.Fixed64(config.MinGenIDRegistrationFee.GetValueAtHeight(height)) {
			return errors.New("registration fee is lower than MinGenIDRegistrationFee")
		}

		txnHash := txn.Hash()
		if txnHash.CompareTo(config.MaxGenerateIDTxnHash.GetValueAtHeight(height)) > 0 {
			return errors.New("txn hash is greater than MaxGenerateIDTxnHash")
		}
	case pb.PayloadType_NANO_PAY_TYPE:
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

	case pb.PayloadType_ISSUE_ASSET_TYPE:
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
			return fmt.Errorf("symbol %s should start with a letter, contain a-z0-9 and have length 3-9", pld.Symbol)
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
