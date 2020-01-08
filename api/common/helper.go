package common

import (
	"context"
	"errors"

	"github.com/gogo/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/vault"
)

func MakeTransferTransaction(wallet vault.Wallet, receipt Uint160, nonce uint64, value, fee Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}

	// construct transaction
	txn, err := transaction.NewTransferAssetTransaction(account.ProgramHash, receipt, nonce, value, fee)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	err = wallet.Sign(txn)
	if err != nil {
		return nil, err
	}

	return txn, nil
}

func MakeSigChainTransaction(wallet vault.Wallet, sigChain []byte, nonce uint64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	txn, err := transaction.NewSigChainTransaction(sigChain, account.ProgramHash, nonce)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	err = wallet.Sign(txn)
	if err != nil {
		return nil, err
	}

	return txn, nil
}

func MakeRegisterNameTransaction(wallet vault.Wallet, name string, nonce uint64, regFee Fixed64, fee Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	registrant := account.PubKey().EncodePoint()
	txn, err := transaction.NewRegisterNameTransaction(registrant, name, nonce, regFee, fee)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	err = wallet.Sign(txn)
	if err != nil {
		return nil, err
	}

	return txn, nil
}

func MakeTransferNameTransaction(wallet vault.Wallet, name string, nonce uint64, fee Fixed64, to []byte) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	registrant := account.PubKey().EncodePoint()
	txn, err := transaction.NewTransferNameTransaction(registrant, to, name, nonce, fee)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	err = wallet.Sign(txn)
	if err != nil {
		return nil, err
	}

	return txn, nil
}

func MakeDeleteNameTransaction(wallet vault.Wallet, name string, nonce uint64, fee Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	registrant := account.PubKey().EncodePoint()
	txn, err := transaction.NewDeleteNameTransaction(registrant, name, nonce, fee)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	err = wallet.Sign(txn)
	if err != nil {
		return nil, err
	}

	return txn, nil
}

func MakeSubscribeTransaction(wallet vault.Wallet, identifier string, topic string, duration uint32, meta string, nonce uint64, fee Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	subscriber := account.PubKey().EncodePoint()
	txn, err := transaction.NewSubscribeTransaction(subscriber, identifier, topic, duration, meta, nonce, fee)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	err = wallet.Sign(txn)
	if err != nil {
		return nil, err
	}

	return txn, nil
}

func MakeUnsubscribeTransaction(wallet vault.Wallet, identifier string, topic string, nonce uint64, fee Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	subscriber := account.PubKey().EncodePoint()
	txn, err := transaction.NewUnsubscribeTransaction(subscriber, identifier, topic, nonce, fee)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	err = wallet.Sign(txn)
	if err != nil {
		return nil, err
	}

	return txn, nil
}

func MakeGenerateIDTransaction(ctx context.Context, wallet vault.Wallet, regFee Fixed64, nonce uint64, txnFee Fixed64, maxTxnHash Uint256) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	pubkey := account.PubKey().EncodePoint()

	var txn *transaction.Transaction
	var txnHash Uint256
	var i uint64
	maxUint64 := ^uint64(0)
	for i = uint64(0); i < maxUint64; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		txn, err = transaction.NewGenerateIDTransaction(pubkey, regFee, nonce, txnFee, proto.EncodeVarint(i))
		if err != nil {
			return nil, err
		}

		txnHash = txn.Hash()
		if txnHash.CompareTo(maxTxnHash) <= 0 {
			break
		}
	}

	if i == maxUint64 {
		return nil, errors.New("No available hash found for all uint64 attrs")
	}

	// sign transaction contract
	err = wallet.Sign(txn)
	if err != nil {
		return nil, err
	}

	return txn, nil
}

func MakeNanoPayTransaction(wallet vault.Wallet, recipient Uint160, id uint64, amount Fixed64, txnExpiration, nanoPayExpiration uint32) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}

	// construct transaction
	txn, err := transaction.NewNanoPayTransaction(account.ProgramHash, recipient, id, amount, txnExpiration, nanoPayExpiration)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	err = wallet.Sign(txn)
	if err != nil {
		return nil, err
	}

	return txn, nil
}

func MakeIssueAssetTransaction(wallet vault.Wallet, name, symbol string, totalSupply Fixed64, precision uint32, nonce uint64, fee Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}

	// construct transaction
	txn, err := transaction.NewIssueAssetTransaction(account.ProgramHash, name, symbol, totalSupply, precision, nonce, fee)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	err = wallet.Sign(txn)
	if err != nil {
		return nil, err
	}

	return txn, nil
}
