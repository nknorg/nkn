package common

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/contract"
	"github.com/nknorg/nkn/types"
	"github.com/nknorg/nkn/vault"
)

func MakeTransferTransaction(wallet vault.Wallet, receipt Uint160, value, fee Fixed64) (*types.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}

	// construct transaction
	txn, err := types.NewTransferAssetTransaction(account.ProgramHash, receipt, value, fee)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeCommitTransaction(wallet vault.Wallet, sigChain []byte) (*types.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	txn, err := types.NewCommitTransaction(sigChain, account.ProgramHash)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeRegisterNameTransaction(wallet vault.Wallet, name string) (*types.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	registrant, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, err
	}
	txn, err := types.NewRegisterNameTransaction(registrant, name)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeDeleteNameTransaction(wallet vault.Wallet, name string) (*types.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	registrant, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, err
	}
	txn, err := types.NewDeleteNameTransaction(registrant, name)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeSubscribeTransaction(wallet vault.Wallet, identifier string, topic string, bucket uint32, duration uint32, meta string) (*types.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	subscriber, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, err
	}
	txn, err := types.NewSubscribeTransaction(subscriber, identifier, topic, bucket, duration, meta)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}
