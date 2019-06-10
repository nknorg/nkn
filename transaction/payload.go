package transaction

import (
	"errors"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/pb"
)

const (
	SubscriptionsLimit      = 1000
	BucketsLimit            = 1000
	MaxSubscriptionDuration = 65535
	MinNanoPayDuration      = 10
)

type IPayload interface {
	Marshal() (data []byte, err error)
	Unmarshal(data []byte) error
}

func Pack(plType pb.PayloadType, payload IPayload) (*pb.Payload, error) {
	data, err := payload.Marshal()
	return &pb.Payload{
		Type: plType,
		Data: data,
	}, err
}

func Unpack(payload *pb.Payload) (IPayload, error) {
	var pl IPayload
	switch payload.Type {
	case pb.CoinbaseType:
		pl = new(pb.Coinbase)
	case pb.TransferAssetType:
		pl = new(pb.TransferAsset)
	case pb.CommitType:
		pl = new(pb.Commit)
	case pb.RegisterNameType:
		pl = new(pb.RegisterName)
	case pb.DeleteNameType:
		pl = new(pb.DeleteName)
	case pb.SubscribeType:
		pl = new(pb.Subscribe)
	case pb.GenerateIDType:
		pl = new(pb.GenerateID)
	case pb.NanoPayType:
		pl = new(pb.NanoPay)
	default:
		return nil, errors.New("invalid payload type.")

	}

	err := pl.Unmarshal(payload.Data)

	return pl, err
}

func NewCoinbase(sender, recipient common.Uint160, amount common.Fixed64) IPayload {
	return &pb.Coinbase{
		Sender:    sender.ToArray(),
		Recipient: recipient.ToArray(),
		Amount:    int64(amount),
	}
}
func NewTransferAsset(sender, recipient common.Uint160, amount common.Fixed64) IPayload {
	return &pb.TransferAsset{
		Sender:    sender.ToArray(),
		Recipient: recipient.ToArray(),
		Amount:    int64(amount),
	}
}

func NewCommit(sigChain []byte, submitter common.Uint160) IPayload {
	return &pb.Commit{
		SigChain:  sigChain,
		Submitter: submitter.ToArray(),
	}
}
func NewRegisterName(registrant []byte, name string) IPayload {
	return &pb.RegisterName{
		Registrant: registrant,
		Name:       name,
	}
}

func NewDeleteName(registrant []byte, name string) IPayload {
	return &pb.DeleteName{
		Registrant: registrant,
		Name:       name,
	}
}

func NewSubscribe(subscriber []byte, id, topic string, bucket, duration uint32, meta string) IPayload {
	return &pb.Subscribe{
		Subscriber: subscriber,
		Identifier: id,
		Topic:      topic,
		Bucket:     bucket,
		Duration:   duration,
		Meta:       meta,
	}
}

func NewGenerateID(publicKey []byte, regFee common.Fixed64) IPayload {
	return &pb.GenerateID{
		PublicKey:       publicKey,
		RegistrationFee: int64(regFee),
	}
}

func NewNanoPay(sender, recipient common.Uint160, nonce uint64, amount common.Fixed64, height, duration uint32) IPayload {
	return &pb.NanoPay{
		Sender:     sender.ToArray(),
		Recipient:  recipient.ToArray(),
		Nonce:      nonce,
		Amount:     int64(amount),
		Height:     height,
		Duration:   duration,
	}
}
