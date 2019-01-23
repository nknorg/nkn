package transaction

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/contract/program"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/crypto/util"
)

const (
	TransactionNonceLength = 32
)

func NewTransferAssetTransaction(sender, recipient Uint160, value, fee Fixed64) (*Transaction, error) {
	payload := &payload.TransferAsset{
		Sender:    sender,
		Recipient: recipient,
		Amount:    value,
	}

	return &Transaction{
		TxType:  TransferAsset,
		Payload: payload,
		Fee:     fee,
		Attributes: []*TxnAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		},
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
		Programs: []*program.Program{},
	}, nil
}

func NewDeleteNameTransaction(registrant []byte, name string) (*Transaction, error) {
	DeleteNamePayload := &payload.DeleteName{
		Registrant: registrant,
		Name:       name,
	}

	return &Transaction{
		TxType:         DeleteName,
		PayloadVersion: 1,
		Payload:        DeleteNamePayload,
		Attributes: []*TxnAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		},
		Programs: []*program.Program{},
	}, nil
}

func NewSubscribeTransaction(subscriber []byte, identifier string, topic string, bucket uint32, duration uint32, meta string) (*Transaction, error) {
	SubscribePayload := &payload.Subscribe{
		Subscriber: subscriber,
		Identifier: identifier,
		Topic:      topic,
		Bucket:     bucket,
		Duration:   duration,
		Meta:       meta,
	}

	return &Transaction{
		TxType:         Subscribe,
		PayloadVersion: 2,
		Payload:        SubscribePayload,
		Attributes: []*TxnAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		},
		Programs: []*program.Program{},
	}, nil
}
