package transaction

import (
	. "nkn-core/common"
	"nkn-core/core/asset"
	"nkn-core/core/contract/program"
	"nkn-core/core/transaction/payload"
	"nkn-core/crypto"
	"nkn-core/crypto/util"
)

const (
	TransactionNonceLength = 32
)

//initial a new transaction with asset registration payload
func NewRegisterAssetTransaction(asset *asset.Asset, amount Fixed64, issuer *crypto.PubKey, conroller Uint160) (*Transaction, error) {
	assetRegPayload := &payload.RegisterAsset{
		Asset:      asset,
		Amount:     amount,
		Issuer:     issuer,
		Controller: conroller,
	}

	return &Transaction{
		UTXOInputs: []*UTXOTxInput{},
		Attributes: []*TxAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		},
		TxType:   RegisterAsset,
		Payload:  assetRegPayload,
		Programs: []*program.Program{},
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
		Attributes: []*TxAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		}, Programs: []*program.Program{},
	}, nil
}

func NewIssueAssetTransaction(outputs []*TxOutput) (*Transaction, error) {
	assetRegPayload := &payload.IssueAsset{}

	return &Transaction{
		TxType:  IssueAsset,
		Payload: assetRegPayload,
		Attributes: []*TxAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		}, Outputs: outputs,
		Programs: []*program.Program{},
	}, nil
}

func NewTransferAssetTransaction(inputs []*UTXOTxInput, outputs []*TxOutput) (*Transaction, error) {
	assetRegPayload := &payload.TransferAsset{}

	return &Transaction{
		TxType:  TransferAsset,
		Payload: assetRegPayload,
		Attributes: []*TxAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		}, UTXOInputs: inputs,
		Outputs:  outputs,
		Programs: []*program.Program{},
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
		TxType:  Prepaid,
		Payload: prepaidPayload,
		Attributes: []*TxAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		}, UTXOInputs: inputs,
		Outputs:  []*TxOutput{changes},
		Programs: []*program.Program{},
	}, nil
}

func NewWithdrawTransaction(output *TxOutput) (*Transaction, error) {
	withdrawPayload := &payload.Withdraw{
		// TODO programhash should be passed in then
		// user could withdraw asset to another address
		ProgramHash: output.ProgramHash,
	}

	return &Transaction{
		TxType:  Withdraw,
		Payload: withdrawPayload,
		Attributes: []*TxAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		}, UTXOInputs: nil,
		Outputs:  []*TxOutput{output},
		Programs: []*program.Program{},
	}, nil
}