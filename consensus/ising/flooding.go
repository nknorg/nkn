package ising

import (
	"io"

	"github.com/nknorg/nkn/core/ledger"
)

type BlockFlooding struct {
	block *ledger.Block
}

func (p *BlockFlooding) Serialize(w io.Writer) error {
	err := p.block.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (p *BlockFlooding) Deserialize(r io.Reader) error {
	block := new(ledger.Block)
	err := block.Deserialize(r)
	if err != nil {
		return err
	}
	p.block = block

	return nil
}

