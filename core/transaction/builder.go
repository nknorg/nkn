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

func NewDeleteNameTransaction(registrant []byte) (*Transaction, error) {
	DeleteNamePayload := &payload.DeleteName{
		Registrant: registrant,
	}

	return &Transaction{
		TxType:  DeleteName,
		Payload: DeleteNamePayload,
		Attributes: []*TxnAttribute{
			{
				Usage: Nonce,
				Data:  util.RandomBytes(TransactionNonceLength),
			},
		},
		Programs: []*program.Program{},
	}, nil
}
