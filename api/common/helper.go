package common

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/contract"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/vault"
)

func MakeTransferTransaction(wallet vault.Wallet, receipt Uint160, value, fee Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}

	// construct transaction
	txn, err := transaction.NewTransferAssetTransaction(account.ProgramHash, receipt, value, fee)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeCommitTransaction(wallet vault.Wallet, sigChain []byte) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	txn, err := transaction.NewCommitTransaction(sigChain, account.ProgramHash)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeRegisterNameTransaction(wallet vault.Wallet, name string) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	registrant, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, err
	}
	txn, err := transaction.NewRegisterNameTransaction(registrant, name)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeDeleteNameTransaction(wallet vault.Wallet, name string) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	registrant, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, err
	}
	txn, err := transaction.NewDeleteNameTransaction(registrant, name)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeSubscribeTransaction(wallet vault.Wallet, identifier string, topic string, bucket uint32, duration uint32, meta string) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	subscriber, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, err
	}
	txn, err := transaction.NewSubscribeTransaction(subscriber, identifier, topic, bucket, duration, meta)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}
