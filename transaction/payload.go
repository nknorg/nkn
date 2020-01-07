package transaction

import (
	"errors"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/pb"
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
	case pb.COINBASE_TYPE:
		pl = new(pb.Coinbase)
	case pb.TRANSFER_ASSET_TYPE:
		pl = new(pb.TransferAsset)
	case pb.SIG_CHAIN_TXN_TYPE:
		pl = new(pb.SigChainTxn)
	case pb.REGISTER_NAME_TYPE:
		pl = new(pb.RegisterName)
	case pb.DELETE_NAME_TYPE:
		pl = new(pb.DeleteName)
	case pb.SUBSCRIBE_TYPE:
		pl = new(pb.Subscribe)
	case pb.UNSUBSCRIBE_TYPE:
		pl = new(pb.Unsubscribe)
	case pb.GENERATE_ID_TYPE:
		pl = new(pb.GenerateID)
	case pb.NANO_PAY_TYPE:
		pl = new(pb.NanoPay)
	case pb.ISSUE_ASSET_TYPE:
		pl = new(pb.IssueAsset)
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

func NewSigChainTxn(sigChain []byte, submitter common.Uint160) IPayload {
	return &pb.SigChainTxn{
		SigChain:  sigChain,
		Submitter: submitter.ToArray(),
	}
}
func NewRegisterName(registrant []byte, name string, fee int64) IPayload {
	return &pb.RegisterName{
		Registrant:      registrant,
		Name:            name,
		RegistrationFee: fee,
	}
}

func NewDeleteName(registrant []byte, name string) IPayload {
	return &pb.DeleteName{
		Registrant: registrant,
		Name:       name,
	}
}

func NewSubscribe(subscriber []byte, id, topic string, duration uint32, meta string) IPayload {
	return &pb.Subscribe{
		Subscriber: subscriber,
		Identifier: id,
		Topic:      topic,
		Duration:   duration,
		Meta:       meta,
	}
}

func NewUnsubscribe(subscriber []byte, id, topic string) IPayload {
	return &pb.Unsubscribe{
		Subscriber: subscriber,
		Identifier: id,
		Topic:      topic,
	}
}

func NewGenerateID(publicKey []byte, regFee common.Fixed64) IPayload {
	return &pb.GenerateID{
		PublicKey:       publicKey,
		RegistrationFee: int64(regFee),
	}
}

func NewNanoPay(sender, recipient common.Uint160, id uint64, amount common.Fixed64, txnExpiration, nanoPayExpiration uint32) IPayload {
	return &pb.NanoPay{
		Sender:            sender.ToArray(),
		Recipient:         recipient.ToArray(),
		Id:                id,
		Amount:            int64(amount),
		TxnExpiration:     txnExpiration,
		NanoPayExpiration: nanoPayExpiration,
	}
}

func NewIssueAsset(sender common.Uint160, name, symbol string, precision uint32, totalSupply common.Fixed64) IPayload {
	return &pb.IssueAsset{
		Sender:      sender.ToArray(),
		Name:        name,
		Symbol:      symbol,
		TotalSupply: int64(totalSupply),
		Precision:   precision,
	}
}
