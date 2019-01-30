package common

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/contract"
	"github.com/nknorg/nkn/types"
	"github.com/nknorg/nkn/vault"
)

func MakeTransferTransaction(wallet vault.Wallet, receipt Uint160, nonce uint64, value, fee Fixed64) (*types.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}

	// construct transaction
	txn, err := types.NewTransferAssetTransaction(account.ProgramHash, receipt, nonce, value, fee)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeCommitTransaction(wallet vault.Wallet, sigChain []byte, nonce uint64) (*types.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	txn, err := types.NewCommitTransaction(sigChain, account.ProgramHash, nonce)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeRegisterNameTransaction(wallet vault.Wallet, name string, nonce uint64) (*types.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	registrant, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, err
	}
	txn, err := types.NewRegisterNameTransaction(registrant, name, nonce)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeDeleteNameTransaction(wallet vault.Wallet, nonce uint64) (*types.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	registrant, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, err
	}
	txn, err := types.NewDeleteNameTransaction(registrant, nonce)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}
