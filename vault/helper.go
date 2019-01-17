package vault

import (
	"errors"
	"fmt"

	. "github.com/nknorg/nkn/common"
	. "github.com/nknorg/nkn/core/asset"
	"github.com/nknorg/nkn/core/contract"
	"github.com/nknorg/nkn/core/transaction"
)

type BatchOut struct {
	Address string
	Value   string
}

func MakeRegTransaction(wallet Wallet, name string, value string) (*transaction.Transaction, error) {
	admin, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	issuer := admin
	asset := &Asset{name, name, MaxPrecision, AssetType(Token)}
	transactionContract, err := contract.CreateSignatureContract(admin.PubKey())
	if err != nil {
		fmt.Println("CreateSignatureContract failed")
		return nil, err
	}
	fixedValue, err := StringToFixed64(value)
	if err != nil {
		return nil, err
	}
	txn, err := transaction.NewRegisterAssetTransaction(asset, fixedValue, issuer.PubKey(), transactionContract.ProgramHash)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeIssueTransaction(wallet Wallet, assetID Uint256, address string, value string) (*transaction.Transaction, error) {
	programHash, err := ToScriptHash(address)
	if err != nil {
		return nil, err
	}
	fixedValue, err := StringToFixed64(value)
	if err != nil {
		return nil, err
	}
	issueTxOutput := &transaction.TxnOutput{
		AssetID:     assetID,
		Value:       fixedValue,
		ProgramHash: programHash,
	}
	outputs := []*transaction.TxnOutput{issueTxOutput}
	txn, err := transaction.NewIssueAssetTransaction(outputs)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeTransferTransaction(wallet Wallet, assetID Uint256, batchOut ...BatchOut) (*transaction.Transaction, error) {
	outputNum := len(batchOut)
	if outputNum == 0 {
		return nil, errors.New("nil outputs")
	}

	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	perOutputFee := Fixed64(0)
	var expected Fixed64
	input := []*transaction.TxnInput{}
	output := []*transaction.TxnOutput{}
	// construct transaction outputs
	for _, o := range batchOut {
		outputValue, err := StringToFixed64(o.Value)
		if err != nil {
			return nil, err
		}
		if outputValue <= perOutputFee {
			return nil, errors.New("token is not enough for transaction fee")
		}
		expected += outputValue
		address, err := ToScriptHash(o.Address)
		if err != nil {
			return nil, errors.New("invalid address")
		}
		tmp := &transaction.TxnOutput{
			AssetID:     assetID,
			Value:       outputValue - perOutputFee,
			ProgramHash: address,
		}
		output = append(output, tmp)
	}

	// construct transaction inputs and changes
	unspent, err := wallet.GetUnspent()
	if err != nil {
		return nil, errors.New("get asset error")
	}
	for _, item := range unspent[assetID] {
		tmpInput := &transaction.TxnInput{
			ReferTxID:          item.Txid,
			ReferTxOutputIndex: uint16(item.Index),
		}
		input = append(input, tmpInput)
		if item.Value > expected {
			changes := &transaction.TxnOutput{
				AssetID:     assetID,
				Value:       item.Value - expected,
				ProgramHash: account.ProgramHash,
			}
			output = append(output, changes)
			expected = 0
			break
		} else if item.Value == expected {
			expected = 0
			break
		} else if item.Value < expected {
			expected = expected - item.Value
		}

	}
	if expected > 0 {
		return nil, errors.New("token is not enough")
	}

	// construct transaction
	txn, err := transaction.NewTransferAssetTransaction(input, output)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakePrepaidTransaction(wallet Wallet, assetID Uint256, value, rates string) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}

	amount, err := StringToFixed64(value)
	if err != nil {
		return nil, err
	}

	unspent, err := wallet.GetUnspent()
	if err != nil {
		return nil, errors.New("get asset error")
	}
	inputs := []*transaction.TxnInput{}
	var changes *transaction.TxnOutput
	for _, item := range unspent[assetID] {
		tmpInput := &transaction.TxnInput{
			ReferTxID:          item.Txid,
			ReferTxOutputIndex: uint16(item.Index),
		}
		inputs = append(inputs, tmpInput)
		if item.Value > amount {
			changes = &transaction.TxnOutput{
				AssetID:     assetID,
				Value:       item.Value - amount,
				ProgramHash: account.ProgramHash,
			}
			amount = 0
			break
		} else if item.Value == amount {
			amount = 0
			break
		} else if item.Value < amount {
			amount = amount - item.Value
		}

	}
	if amount > 0 {
		return nil, errors.New("token is not enough")
	}

	txn, err := transaction.NewPrepaidTransaction(inputs, changes, assetID, value, rates)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeWithdrawTransaction(wallet Wallet, assetID Uint256, value string) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}

	amount, err := StringToFixed64(value)
	if err != nil {
		return nil, err
	}

	output := &transaction.TxnOutput{
		AssetID:     assetID,
		Value:       amount,
		ProgramHash: account.ProgramHash,
	}

	txn, err := transaction.NewWithdrawTransaction(output)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeCommitTransaction(wallet Wallet, sigChain []byte) (*transaction.Transaction, error) {
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

func MakeRegisterNameTransaction(wallet Wallet, name string) (*transaction.Transaction, error) {
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

func MakeDeleteNameTransaction(wallet Wallet, name string) (*transaction.Transaction, error) {
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
