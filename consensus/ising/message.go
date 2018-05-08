package ising

import (
	"bytes"
	"errors"

	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/crypto"
)

type IsingMessageType byte

const (
	BlockFloodingMsg IsingMessageType = 0x00
	BlockRequestMsg  IsingMessageType = 0x01
	BlockResponseMsg IsingMessageType = 0x02
	BlockProposalMsg IsingMessageType = 0x03
	BlockVoteMsg     IsingMessageType = 0x04
	StateProbeMsg    IsingMessageType = 0x05
	StateResponseMsg IsingMessageType = 0x06
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
	case *Voting:
		err = serialization.WriteByte(buf, byte(BlockVoteMsg))
	case *StateProbe:
		err = serialization.WriteByte(buf, byte(StateProbeMsg))
	case *StateResponse:
		err = serialization.WriteByte(buf, byte(StateResponseMsg))
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
	case BlockVoteMsg:
		bvmsg := &Voting{}
		err := bvmsg.Deserialize(r)
		if err != nil {
			return nil, err
		}
		return bvmsg, nil
	case StateProbeMsg:
		spmsg := &StateProbe{}
		err := spmsg.Deserialize(r)
		if err != nil {
			return nil, err
		}
		return spmsg, nil
	case StateResponseMsg:
		srmsg := &StateResponse{}
		err := srmsg.Deserialize(r)
		if err != nil {
			return nil, err
		}
		return srmsg, nil
	}

	return nil, errors.New("invalid ising consensus message.")
}
