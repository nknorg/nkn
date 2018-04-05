package ising

import (
	"bytes"
	"errors"

	"nkn-core/common/serialization"
	"nkn-core/net/message"
)

type IsingMessageType byte

const (
	BlockFloodingMsg IsingMessageType = 0x00
	BlockRequestMsg  IsingMessageType = 0x01
	BlockProposalMsg IsingMessageType = 0x02
	BlockVoteMsg     IsingMessageType = 0x03
)

type IsingMessage interface {
	serialization.SerializableData
}

func BuildIsingPayload(s serialization.SerializableData) (*message.IsingPayload, error) {
	var err error
	buf := bytes.NewBuffer(nil)
	switch s.(type) {
	case *BlockFlooding:
		err = serialization.WriteByte(buf, byte(BlockFloodingMsg))
	case *BlockRequest:
		err = serialization.WriteByte(buf, byte(BlockRequestMsg))
	case *BlockProposal:
		err = serialization.WriteByte(buf, byte(BlockProposalMsg))
	case *BlockVote:
		err = serialization.WriteByte(buf, byte(BlockVoteMsg))
	}
	if err != nil {
		return nil, err
	}
	err = s.Serialize(buf)
	if err != nil {
		return nil, err
	}
	payload := &message.IsingPayload{
		PayloadData: buf.Bytes(),
	}

	return payload, nil
}

func RecoverFromIsingPayload(data []byte) (IsingMessage, error) {
	r := bytes.NewReader(data)
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
		brmsg := &BlockRequest{}
		err := brmsg.Deserialize(r)
		if err != nil {
			return nil, err
		}
		return brmsg, nil
	case BlockProposalMsg:
		bpmsg := &BlockProposal{}
		err := bpmsg.Deserialize(r)
		if err != nil {
			return nil, err
		}
		return bpmsg, nil
	case BlockVoteMsg:
		bvmsg := &BlockVote{}
		err := bvmsg.Deserialize(r)
		if err != nil {
			return nil, err
		}
	}

	return nil, errors.New("invalid ising consensus message.")
}