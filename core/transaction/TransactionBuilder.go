package transaction

import (
	. "nkn-core/common"
	"nkn-core/core/asset"
	"nkn-core/core/code"
	"nkn-core/core/contract/program"
	"nkn-core/core/transaction/payload"
	"nkn-core/crypto"
	"nkn-core/smartcontract/types"
)

//initial a new transaction with asset registration payload
func NewRegisterAssetTransaction(asset *asset.Asset, amount Fixed64, issuer *crypto.PubKey, conroller Uint160) (*Transaction, error) {

	//TODO: check arguments

	assetRegPayload := &payload.RegisterAsset{
		Asset:  asset,
		Amount: amount,
		//Precision: precision,
		Issuer:     issuer,
		Controller: conroller,
	}

	return &Transaction{
		//nonce uint64 //TODO: genenrate nonce
		UTXOInputs: []*UTXOTxInput{},
		Attributes: []*TxAttribute{},
		TxType:     RegisterAsset,
		Payload:    assetRegPayload,
		Programs:   []*program.Program{},
	}, nil
}

//initial a new transaction with asset registration payload
func NewBookKeeperTransaction(pubKey *crypto.PubKey, isAdd bool, cert []byte, issuer *crypto.PubKey) (*Transaction, error) {

	bookKeeperPayload := &payload.BookKeeper{
		PubKey: pubKey,
		Action: payload.BookKeeperAction_SUB,
		Cert:   cert,
		Issuer: issuer,
	}

	if isAdd {
		bookKeeperPayload.Action = payload.BookKeeperAction_ADD
	}

	return &Transaction{
		TxType:     BookKeeper,
		Payload:    bookKeeperPayload,
		UTXOInputs: []*UTXOTxInput{},
		Attributes: []*TxAttribute{},
		Programs:   []*program.Program{},
	}, nil
}

func NewIssueAssetTransaction(outputs []*TxOutput) (*Transaction, error) {

	assetRegPayload := &payload.IssueAsset{}

	return &Transaction{
		TxType:     IssueAsset,
		Payload:    assetRegPayload,
		Attributes: []*TxAttribute{},
		Outputs:    outputs,
		Programs:   []*program.Program{},
	}, nil
}

func NewTransferAssetTransaction(inputs []*UTXOTxInput, outputs []*TxOutput) (*Transaction, error) {

	//TODO: check arguments

	assetRegPayload := &payload.TransferAsset{}

	return &Transaction{
		TxType:     TransferAsset,
		Payload:    assetRegPayload,
		Attributes: []*TxAttribute{},
		UTXOInputs: inputs,
		Outputs:    outputs,
		Programs:   []*program.Program{},
	}, nil
}

func NewPrepaidTransaction(inputs []*UTXOTxInput, changes *TxOutput, assetID Uint256, amount, rates string) (*Transaction, error) {
	a, err := StringToFixed64(amount)
	if err != nil {
		return nil, err
	}
	r, err := StringToFixed64(rates)
	if err != nil {
		return nil, err
	}
	prepaidPayload := &payload.Prepaid{
		Asset:  assetID,
		Amount: a,
		Rates:  r,
	}

	return &Transaction{
		TxType:     Prepaid,
		Payload:    prepaidPayload,
		Attributes: []*TxAttribute{},
		UTXOInputs: inputs,
		Outputs:    []*TxOutput{changes},
		Programs:   []*program.Program{},
	}, nil
}

func NewWithdrawTransaction(output *TxOutput) (*Transaction, error) {
	withdrawPayload := &payload.Withdraw{
		// TODO programhash should be passed in then
		// user could withdraw asset to another address
		ProgramHash: output.ProgramHash,
	}

	return &Transaction{
		TxType:     Withdraw,
		Payload:    withdrawPayload,
		Attributes: []*TxAttribute{},
		UTXOInputs: nil,
		Outputs:    []*TxOutput{output},
		Programs:   []*program.Program{},
	}, nil
}

//initial a new transaction with publish payload
func NewDeployTransaction(fc *code.FunctionCode, programHash Uint160, name, codeversion, author, email, desp string, language types.LangType) (*Transaction, error) {
	//TODO: check arguments
	DeployCodePayload := &payload.DeployCode{
		Code:        fc,
		Name:        name,
		CodeVersion: codeversion,
		Author:      author,
		Email:       email,
		Description: desp,
		Language:    language,
		ProgramHash: programHash,
	}

	return &Transaction{
		TxType:     DeployCode,
		Payload:    DeployCodePayload,
		Attributes: []*TxAttribute{},
		UTXOInputs: []*UTXOTxInput{},
		Programs:   []*program.Program{},
	}, nil
}

//initial a new transaction with invoke payload
func NewInvokeTransaction(fc []byte, codeHash Uint160, programhash Uint160) (*Transaction, error) {
	//TODO: check arguments
	InvokeCodePayload := &payload.InvokeCode{
		Code:        fc,
		CodeHash:    codeHash,
		ProgramHash: programhash,
	}

	return &Transaction{
		TxType:     InvokeCode,
		Payload:    InvokeCodePayload,
		Attributes: []*TxAttribute{},
		UTXOInputs: []*UTXOTxInput{},
		Programs:   []*program.Program{},
	}, nil
}
