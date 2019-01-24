package types

import (
	"errors"

	"github.com/nknorg/nkn/common"
)

type IPayload interface {
	Marshal() (dAtA []byte, err error)
	Unmarshal(dAtA []byte) error
}

func Pack(plType PayloadType, payload IPayload) (*Payload, error) {
	data, err := payload.Marshal()
	return &Payload{
		Type: plType,
		Data: data,
	}, err
}

func Unpack(payload *Payload) (IPayload, error) {
	var pl IPayload
	switch payload.Type {
	case CoinbaseType:
		pl = new(Coinbase)
	case TransferAssetType:
		pl = new(TransferAsset)
	case CommitType:
		pl = new(Commit)
	case RegisterNameType:
		pl = new(RegisterName)
	case DeleteNameType:
		pl = new(DeleteName)
	case SubscribeType:
		pl = new(Subscribe)
	default:
		return nil, errors.New("invalid payload type.")

	}

	err := pl.Unmarshal(payload.Data)

	return pl, err
}

func NewCoinbase(sender, recipient common.Uint160, amount common.Fixed64) IPayload {
	return &Coinbase{
		Sender:    sender.ToArray(),
		Recipient: recipient.ToArray(),
		Amount:    int64(amount),
	}
}
func NewTransferAsset(sender, recipient common.Uint160, amount common.Fixed64) IPayload {
	return &TransferAsset{
		Sender:    sender.ToArray(),
		Recipient: recipient.ToArray(),
		Amount:    int64(amount),
	}
}

func NewCommit(sigChain []byte, submitter common.Uint160) IPayload {
	return &Commit{
		SigChain:  sigChain,
		Submitter: submitter.ToArray(),
	}
}
func NewRegisterName(registrant []byte, name string) IPayload {
	return &RegisterName{
		Registrant: registrant,
		Name:       name,
	}
}

func NewDeleteName(registrant []byte, name string) IPayload {
	return &DeleteName{
		Registrant: registrant,
		Name:       name,
	}
}

func NewSubscribe(subscriber []byte, id, topic string, bucket, duration uint32, meta string) IPayload {
	return &Subscribe{
		Subscriber: subscriber,
		Identifier: id,
		Topic:      topic,
		Bucket:     bucket,
		Duration:   duration,
		Meta:       meta,
	}
}
