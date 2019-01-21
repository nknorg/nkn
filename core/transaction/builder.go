package transaction

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/asset"
	"github.com/nknorg/nkn/core/contract/program"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/crypto/util"
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
		Inputs: []*TxnInput{},
		Attributes: []*TxnAttribute{
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

func NewIssueAssetTransaction(outputs []*TxnOutput) (*Transaction, error) {
	assetRegPayload := &payload.IssueAsset{}

	return &Transaction{
		TxType:  IssueAsset,
		Payload: assetRegPayload,
		Attributes: []*TxnAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		},
		Outputs:  outputs,
		Programs: []*program.Program{},
	}, nil
}

func NewTransferAssetTransaction(inputs []*TxnInput, outputs []*TxnOutput) (*Transaction, error) {
	assetRegPayload := &payload.TransferAsset{}

	return &Transaction{
		TxType:  TransferAsset,
		Payload: assetRegPayload,
		Attributes: []*TxnAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		},
		Inputs:   inputs,
		Outputs:  outputs,
		Programs: []*program.Program{},
	}, nil
}

func NewPrepaidTransaction(inputs []*TxnInput, changes *TxnOutput, assetID Uint256, amount, rates string) (*Transaction, error) {
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
		Attributes: []*TxnAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		},
		Inputs:   inputs,
		Outputs:  []*TxnOutput{changes},
		Programs: []*program.Program{},
	}, nil
}

func NewWithdrawTransaction(output *TxnOutput) (*Transaction, error) {
	withdrawPayload := &payload.Withdraw{
		// TODO programhash should be passed in then
		// user could withdraw asset to another address
		ProgramHash: output.ProgramHash,
	}

	return &Transaction{
		TxType:  Withdraw,
		Payload: withdrawPayload,
		Attributes: []*TxnAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		},
		Inputs:   nil,
		Outputs:  []*TxnOutput{output},
		Programs: []*program.Program{},
	}, nil
}

func NewCommitTransaction(sigChain []byte, submitter Uint160) (*Transaction, error) {
	CommitPayload := &payload.Commit{
		SigChain:  sigChain,
		Submitter: submitter,
	}

	return &Transaction{
		TxType:  Commit,
		Payload: CommitPayload,
		Attributes: []*TxnAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		},
		Inputs:   nil,
		Programs: []*program.Program{},
	}, nil
}

func NewRegisterNameTransaction(registrant []byte, name string) (*Transaction, error) {
	RegisterNamePayload := &payload.RegisterName{
		Registrant: registrant,
		Name:       name,
	}

	return &Transaction{
		TxType:  RegisterName,
		Payload: RegisterNamePayload,
		Attributes: []*TxnAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		},
		Inputs:   nil,
		Programs: []*program.Program{},
	}, nil
}

func NewDeleteNameTransaction(registrant []byte, name string) (*Transaction, error) {
	DeleteNamePayload := &payload.DeleteName{
		Registrant: registrant,
		Name:       name,
	}

	return &Transaction{
		TxType:  DeleteName,
		PayloadVersion: 1,
		Payload: DeleteNamePayload,
		Attributes: []*TxnAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		},
		Inputs:   nil,
		Programs: []*program.Program{},
	}, nil
}

func NewSubscribeTransaction(subscriber []byte, identifier string, topic string, bucket uint32, duration uint32, meta string) (*Transaction, error) {
	SubscribePayload := &payload.Subscribe{
		Subscriber: subscriber,
		Identifier: identifier,
		Topic: topic,
		Bucket: bucket,
		Duration: duration,
		Meta: meta,
	}

	return &Transaction{
		TxType:  Subscribe,
		PayloadVersion: 2,
		Payload: SubscribePayload,
		Attributes: []*TxnAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		},
		Inputs:   nil,
		Programs: []*program.Program{},
	}, nil
}
