package ising

import (
	"io"

	. "nkn-core/common"
	"nkn-core/crypto"
	"nkn-core/common/serialization"
)


type BlockProposal struct {
	blockHash *Uint256
	proposer  *crypto.PubKey
	signature [32]byte
}

func (p *BlockProposal) Serialize(w io.Writer) error {
	_, err := p.blockHash.Serialize(w)
	if err != nil {
		return err
	}
	err = p.proposer.Serialize(w)
	if err != nil {
		return err
	}
	err = serialization.WriteVarBytes(w, p.signature[:])
	if err != nil {
		return err
	}

	return nil
}

func (p *BlockProposal) Deserialize(r io.Reader) error {
	err := p.blockHash.Deserialize(r)
	if err != nil {
		return err
	}
	err = p.proposer.DeSerialize(r)
	if err != nil {
		return err
	}
	signature, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	copy(p.signature[:], signature)

	return nil
}

