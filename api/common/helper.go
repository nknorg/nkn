package common

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/vault"
	"github.com/nknorg/nkn/vm/contract"
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
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeCommitTransaction(wallet vault.Wallet, sigChain []byte, nonce uint64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	txn, err := transaction.NewCommitTransaction(sigChain, account.ProgramHash, nonce)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeRegisterNameTransaction(wallet vault.Wallet, name string, nonce uint64, fee Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	registrant, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, err
	}
	txn, err := transaction.NewRegisterNameTransaction(registrant, name, nonce, fee)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeDeleteNameTransaction(wallet vault.Wallet, name string, nonce uint64, fee Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	registrant, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, err
	}
	txn, err := transaction.NewDeleteNameTransaction(registrant, name, nonce, fee)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeSubscribeTransaction(wallet vault.Wallet, identifier string, topic string, bucket uint32, duration uint32, meta string, nonce uint64, fee Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	subscriber, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, err
	}
	txn, err := transaction.NewSubscribeTransaction(subscriber, identifier, topic, bucket, duration, meta, nonce, fee)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeGenerateIDTransaction(wallet vault.Wallet, regFee Fixed64, nonce uint64, txnFee Fixed64) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	pubkey, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, err
	}
	txn, err := transaction.NewGenerateIDTransaction(pubkey, regFee, nonce, txnFee)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeNanoPayTransaction(wallet vault.Wallet, recipient Uint160, nonce uint64, amount Fixed64, height, duration uint32) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}

	// construct transaction
	txn, err := transaction.NewNanoPayTransaction(account.ProgramHash, recipient, nonce, amount, height, duration)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}
