package ledger

import (
	"nkn-core/common"
	"nkn-core/core/code"
	"nkn-core/core/contract/program"
	"nkn-core/core/ledger/payload"
	"nkn-core/smartcontract/types"
)

func NewTransferAssetTransaction(inputs []*UTXOTxInput, outputs []*TxOutput) (*Transaction, error) {
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

//initial a new transaction with publish payload
func NewDeployTransaction(fc *code.FunctionCode, programHash common.Uint160, name, codeversion, author, email, desp string, language types.LangType) (*Transaction, error) {
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
func NewInvokeTransaction(fc []byte, codeHash common.Uint160, programhash common.Uint160) (*Transaction, error) {
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
