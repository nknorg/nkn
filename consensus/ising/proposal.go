package ising

import (
	"io"

	. "github.com/nknorg/nkn/common"
)


type BlockProposal struct {
	blockHash *Uint256
}

func (p *BlockProposal) Serialize(w io.Writer) error {
	_, err := p.blockHash.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (p *BlockProposal) Deserialize(r io.Reader) error {
	p.blockHash = new(Uint256)
	err := p.blockHash.Deserialize(r)
	if err != nil {
		return err
	}

	return nil
}

