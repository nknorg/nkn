package transaction

import (
	"errors"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/pb"
)

func Pack(plType pb.PayloadType, payload proto.Message) (*pb.Payload, error) {
	data, err := proto.Marshal(payload)
	return &pb.Payload{
		Type: plType,
		Data: data,
	}, err
}

func Unpack(payload *pb.Payload) (proto.Message, error) {
	var m proto.Message
	switch payload.Type {
	case pb.PayloadType_COINBASE_TYPE:
		m = new(pb.Coinbase)
	case pb.PayloadType_TRANSFER_ASSET_TYPE:
		m = new(pb.TransferAsset)
	case pb.PayloadType_SIG_CHAIN_TXN_TYPE:
		m = new(pb.SigChainTxn)
	case pb.PayloadType_REGISTER_NAME_TYPE:
		m = new(pb.RegisterName)
	case pb.PayloadType_TRANSFER_NAME_TYPE:
		m = new(pb.TransferName)
	case pb.PayloadType_DELETE_NAME_TYPE:
		m = new(pb.DeleteName)
	case pb.PayloadType_SUBSCRIBE_TYPE:
		m = new(pb.Subscribe)
	case pb.PayloadType_UNSUBSCRIBE_TYPE:
		m = new(pb.Unsubscribe)
	case pb.PayloadType_GENERATE_ID_TYPE:
		m = new(pb.GenerateID)
	case pb.PayloadType_NANO_PAY_TYPE:
		m = new(pb.NanoPay)
	case pb.PayloadType_ISSUE_ASSET_TYPE:
		m = new(pb.IssueAsset)
	default:
		return nil, errors.New("invalid payload type")
	}

	err := proto.Unmarshal(payload.Data, m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func NewCoinbase(sender, recipient common.Uint160, amount common.Fixed64) *pb.Coinbase {
	return &pb.Coinbase{
		Sender:    sender.ToArray(),
		Recipient: recipient.ToArray(),
		Amount:    int64(amount),
	}
}

func NewTransferAsset(sender, recipient common.Uint160, amount common.Fixed64) *pb.TransferAsset {
	return &pb.TransferAsset{
		Sender:    sender.ToArray(),
		Recipient: recipient.ToArray(),
		Amount:    int64(amount),
	}
}

func NewSigChainTxn(sigChain []byte, submitter common.Uint160) *pb.SigChainTxn {
	return &pb.SigChainTxn{
		SigChain:  sigChain,
		Submitter: submitter.ToArray(),
	}
}

func NewRegisterName(registrant []byte, name string, fee int64) *pb.RegisterName {
	return &pb.RegisterName{
		Registrant:      registrant,
		Name:            name,
		RegistrationFee: fee,
	}
}

func NewTransferName(registrant []byte, receipt []byte, name string) *pb.TransferName {
	return &pb.TransferName{
		Registrant: registrant,
		Recipient:  receipt,
		Name:       name,
	}
}

func NewDeleteName(registrant []byte, name string) *pb.DeleteName {
	return &pb.DeleteName{
		Registrant: registrant,
		Name:       name,
	}
}

func NewSubscribe(subscriber []byte, id, topic string, duration uint32, meta string) *pb.Subscribe {
	return &pb.Subscribe{
		Subscriber: subscriber,
		Identifier: id,
		Topic:      topic,
		Duration:   duration,
		Meta:       []byte(meta),
	}
}

func NewUnsubscribe(subscriber []byte, id, topic string) *pb.Unsubscribe {
	return &pb.Unsubscribe{
		Subscriber: subscriber,
		Identifier: id,
		Topic:      topic,
	}
}

func NewGenerateID(publicKey, sender []byte, regFee common.Fixed64, version int32) *pb.GenerateID {
	return &pb.GenerateID{
		PublicKey:       publicKey,
		Sender:          sender,
		RegistrationFee: int64(regFee),
		Version:         version,
	}
}

func NewNanoPay(sender, recipient common.Uint160, id uint64, amount common.Fixed64, txnExpiration, nanoPayExpiration uint32) *pb.NanoPay {
	return &pb.NanoPay{
		Sender:            sender.ToArray(),
		Recipient:         recipient.ToArray(),
		Id:                id,
		Amount:            int64(amount),
		TxnExpiration:     txnExpiration,
		NanoPayExpiration: nanoPayExpiration,
	}
}

func NewIssueAsset(sender common.Uint160, name, symbol string, precision uint32, totalSupply common.Fixed64) *pb.IssueAsset {
	return &pb.IssueAsset{
		Sender:      sender.ToArray(),
		Name:        name,
		Symbol:      symbol,
		TotalSupply: int64(totalSupply),
		Precision:   precision,
	}
}
