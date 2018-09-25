package ising

import (
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type Pong struct {
	blockHash   Uint256 // block hash
	blockHeight uint32  // current height if pinged height doesn't exist
	pingHeight  uint32  // response to which ping
}

func NewPong(hash Uint256, height uint32, pingHeight uint32) *Pong {
	return &Pong{
		blockHash:   hash,
		blockHeight: height,
		pingHeight:  pingHeight,
	}
}

func (p *Pong) Serialize(w io.Writer) error {
	var err error
	_, err = p.blockHash.Serialize(w)
	if err != nil {
		return err
	}
	err = serialization.WriteUint32(w, p.blockHeight)
	if err != nil {
		return err
	}
	err = serialization.WriteUint32(w, p.pingHeight)
	if err != nil {
		return err
	}

	return nil
}

func (p *Pong) Deserialize(r io.Reader) error {
	var currentHash Uint256
	err := currentHash.Deserialize(r)
	if err != nil {
		return nil
	}
	p.blockHash = currentHash

	blockHeight, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	p.blockHeight = blockHeight

	pingHeight, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	p.pingHeight = pingHeight

	return nil
}
