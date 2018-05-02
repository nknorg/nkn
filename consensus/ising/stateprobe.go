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

func (p *StateProbe) Serialize(w io.Writer) error {
	var err error
	err = serialization.WriteByte(w, byte(p.ProbeType))
	if err != nil {
		return err
	}
	if p.ProbePayload != nil {
		switch t := p.ProbePayload.(type) {
		case BlockHistoryPayload:
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

func (p *StateProbe) Deserialize(r io.Reader) error {
	var err error
	t, err := serialization.ReadByte(r)
	if err != nil {
		return err
	}
	p.ProbeType = ProbeType(t)
	switch p.ProbeType {
	case BlockHistory:
		startHeight, err := serialization.ReadUint32(r)
		if err != nil {
			return err
		}
		blockNum, err := serialization.ReadUint32(r)
		if err != nil {
			return err
		}
		p.ProbePayload = &BlockHistoryPayload{
			StartHeight: startHeight,
			BlockNum:    blockNum,
		}
	}

	return nil
}
