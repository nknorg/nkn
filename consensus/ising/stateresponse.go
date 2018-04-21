package ising

import (
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type StateResponse struct {
	currentBlockHash   Uint256
	currentBlockHeight uint32
}

func (p *StateResponse) Serialize(w io.Writer) error {
	var err error
	_, err = p.currentBlockHash.Serialize(w)
	if err != nil {
		return err
	}
	serialization.WriteUint32(w, p.currentBlockHeight)

	return nil
}

func (p *StateResponse) Deserialize(r io.Reader) error {
	var err error
	err = p.currentBlockHash.Deserialize(r)
	if err != nil {
		return err
	}
	height, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	p.currentBlockHeight = height

	return nil
}
