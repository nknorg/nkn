package ising

import (
	"io"

	"github.com/nknorg/nkn/common/serialization"
)

type ProbeType byte

const (
	// detected block history of neighbors
	BlockHistory ProbeType = iota
)

// Contains block history [StartHeight, StartHeight + BlockNum) to be detected
type BlockHistoryPayload struct {
	StartHeight uint32
	BlockNum    uint32
}

type StateProbe struct {
	ProbeType    ProbeType
	ProbePayload interface{}
}

func (sp *StateProbe) Serialize(w io.Writer) error {
	var err error
	err = serialization.WriteByte(w, byte(sp.ProbeType))
	if err != nil {
		return err
	}
	if sp.ProbePayload != nil {
		switch t := sp.ProbePayload.(type) {
		case *BlockHistoryPayload:
			err = serialization.WriteUint32(w, t.StartHeight)
			if err != nil {
				return err
			}
			err = serialization.WriteUint32(w, t.BlockNum)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (sp *StateProbe) Deserialize(r io.Reader) error {
	var err error
	t, err := serialization.ReadByte(r)
	if err != nil {
		return err
	}
	sp.ProbeType = ProbeType(t)
	switch sp.ProbeType {
	case BlockHistory:
		startHeight, err := serialization.ReadUint32(r)
		if err != nil {
			return err
		}
		blockNum, err := serialization.ReadUint32(r)
		if err != nil {
			return err
		}
		sp.ProbePayload = &BlockHistoryPayload{
			StartHeight: startHeight,
			BlockNum:    blockNum,
		}
	}

	return nil
}
