package ising

import (
	"io"

	"github.com/nknorg/nkn/core/ledger"
)

type BlockFlooding struct {
	block *ledger.Block
}

func NewBlockFlooding(block *ledger.Block) *BlockFlooding {
	return &BlockFlooding{
		block: block,
	}
}

func (bf *BlockFlooding) Serialize(w io.Writer) error {
	err := bf.block.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (bf *BlockFlooding) Deserialize(r io.Reader) error {
	block := new(ledger.Block)
	err := block.Deserialize(r)
	if err != nil {
		return err
	}
	bf.block = block

	return nil
}
