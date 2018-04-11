package ising

import (
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type BlockVote struct {
	blockHash *Uint256
	agree bool
}

func (p *BlockVote) Serialize(w io.Writer) error {
	_, err := p.blockHash.Serialize(w)
	if err != nil {
		return err
	}
	err = serialization.WriteBool(w, p.agree)
	if err != nil {
		return err
	}

	return nil
}

func (p *BlockVote) Deserialize(r io.Reader) error {
	p.blockHash = new(Uint256)
	err := p.blockHash.Deserialize(r)
	if err != nil {
		return err
	}
	agree, err := serialization.ReadBool(r)
	if err != nil {
		return err
	}
	p.agree = agree

	return nil
}

