package ising

import (
	"bytes"
	"errors"

	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/net/message"
)

type IsingMessageType byte

const (
	BlockFloodingMsg IsingMessageType = 0x00
	BlockRequestMsg  IsingMessageType = 0x01
	BlockResponseMsg IsingMessageType = 0x02
	BlockProposalMsg IsingMessageType = 0x03
	MindChangingMsg  IsingMessageType = 0x04
	PingMsg          IsingMessageType = 0x07
	PongMsg          IsingMessageType = 0x08
)

type IsingMessage interface {
	serialization.SerializableData
}

func BuildIsingPayload(msg IsingMessage, sender *crypto.PubKey) (*message.IsingPayload, error) {
	var err error
	buf := bytes.NewBuffer(nil)
	switch msg.(type) {
	case *BlockFlooding:
		err = serialization.WriteByte(buf, byte(BlockFloodingMsg))
	case *Request:
		err = serialization.WriteByte(buf, byte(BlockRequestMsg))
	case *Response:
		err = serialization.WriteByte(buf, byte(BlockResponseMsg))
	case *Proposal:
		err = serialization.WriteByte(buf, byte(BlockProposalMsg))
	case *MindChanging:
		err = serialization.WriteByte(buf, byte(MindChangingMsg))
	case *Ping:
		err = serialization.WriteByte(buf, byte(PingMsg))
	case *Pong:
		err = serialization.WriteByte(buf, byte(PongMsg))
	}
	if err != nil {
		return nil, err
	}
	err = msg.Serialize(buf)
	if err != nil {
		return nil, err
	}
	payload := &message.IsingPayload{
		PayloadData: buf.Bytes(),
		Sender:      sender,
		Signature:   nil,
	}

	return payload, nil
}

func RecoverFromIsingPayload(payload *message.IsingPayload) (IsingMessage, error) {
	r := bytes.NewReader(payload.PayloadData)
	msgType, err := serialization.ReadByte(r)
	if err != nil {
		return nil, err
	}
	mtype := IsingMessageType(msgType)
	switch mtype {
	case BlockFloodingMsg:
		bfmsg := &BlockFlooding{}
		err := bfmsg.Deserialize(r)
		if err != nil {
			return nil, err
		}
		return bfmsg, nil
	case BlockRequestMsg:
		brmsg := &Request{}
		err := brmsg.Deserialize(r)
		if err != nil {
			return nil, err
		}
		return brmsg, nil
	case BlockResponseMsg:
		brmsg := &Response{}
		err := brmsg.Deserialize(r)
		if err != nil {
			return nil, err
		}
		return brmsg, nil
	case BlockProposalMsg:
		bpmsg := &Proposal{}
		err := bpmsg.Deserialize(r)
		if err != nil {
			return nil, err
		}
		return bpmsg, nil
	case MindChangingMsg:
		mcMsg := &MindChanging{}
		err := mcMsg.Deserialize(r)
		if err != nil {
			return nil, err
		}
		return mcMsg, nil
	case PingMsg:
		pingMsg := &Ping{}
		err := pingMsg.Deserialize(r)
		if err != nil {
			return nil, err
		}
		return pingMsg, nil
	case PongMsg:
		pongMsg := &Pong{}
		err := pongMsg.Deserialize(r)
		if err != nil {
			return nil, err
		}
		return pongMsg, nil
	}

	return nil, errors.New("invalid ising consensus message.")
}
