package common

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/nknorg/nkn/v2/config"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/transaction"
	"github.com/nknorg/nkn/v2/vault"
)

func MakeTransferTransaction(wallet *vault.Wallet, receipt common.Uint160, nonce uint64, value, fee common.Fixed64) (*transaction.Transaction, error) {
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

func MakeSigChainTransaction(wallet *vault.Wallet, sigChain []byte, nonce uint64) (*transaction.Transaction, error) {
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

func MakeRegisterNameTransaction(wallet *vault.Wallet, name string, nonce uint64, regFee common.Fixed64, fee common.Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	registrant := account.PubKey()
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

func MakeTransferNameTransaction(wallet *vault.Wallet, name string, nonce uint64, fee common.Fixed64, to []byte) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	registrant := account.PubKey()
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

func MakeDeleteNameTransaction(wallet *vault.Wallet, name string, nonce uint64, fee common.Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	registrant := account.PubKey()
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

func MakeSubscribeTransaction(wallet *vault.Wallet, identifier string, topic string, duration uint32, meta string, nonce uint64, fee common.Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	subscriber := account.PubKey()
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

func MakeUnsubscribeTransaction(wallet *vault.Wallet, identifier string, topic string, nonce uint64, fee common.Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	subscriber := account.PubKey()
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

func MakeGenerateIDTransaction(ctx context.Context, pubkey []byte, wallet *vault.Wallet, regFee common.Fixed64, nonce uint64, txnFee common.Fixed64, height uint32) (*transaction.Transaction, error) {
	minVersion := config.AllowTxnGenerateIDMinVersion.GetValueAtHeight(height + 1)
	maxVersion := config.AllowTxnGenerateIDMaxVersion.GetValueAtHeight(height + 1)
	if maxVersion < minVersion {
		return nil, fmt.Errorf("no available ID version at height %d", height+1)
	}

	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	myPubkey := account.PubKey()

	if len(pubkey) == 0 {
		pubkey = myPubkey
	}

	var sender []byte
	if config.AllowGenerateIDSender.GetValueAtHeight(height + 1) {
		sender = account.ProgramHash.ToArray()
	} else {
		if !bytes.Equal(pubkey, myPubkey) {
			return nil, errors.New("cannot generate ID for another pubkey at this height")
		}
	}

	if regFee == 0 {
		regFee = common.Fixed64(config.MinGenIDRegistrationFee.GetValueAtHeight(height + 1))
	}
	maxTxnHash := config.MaxGenerateIDTxnHash.GetValueAtHeight(height + 1)

	var txn *transaction.Transaction
	var txnHash common.Uint256
	var i uint64
	maxUint64 := ^uint64(0)

	for i = uint64(0); i < maxUint64; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		txn, err = transaction.NewGenerateIDTransaction(pubkey, sender, regFee, maxVersion, nonce, txnFee, proto.EncodeVarint(i))
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

func MakeNanoPayTransaction(wallet *vault.Wallet, recipient common.Uint160, id uint64, amount common.Fixed64, txnExpiration, nanoPayExpiration uint32) (*transaction.Transaction, error) {
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

func MakeIssueAssetTransaction(wallet *vault.Wallet, name, symbol string, totalSupply common.Fixed64, precision uint32, nonce uint64, fee common.Fixed64) (*transaction.Transaction, error) {
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
