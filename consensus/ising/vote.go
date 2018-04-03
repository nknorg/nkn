package ising

import (
	"io"

	. "nkn-core/common"
	"nkn-core/crypto"
	"nkn-core/common/serialization"
)

type BlockVote struct {
	blockHash *Uint256
	agree bool
	voter *crypto.PubKey
	signature [32]byte
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
	err = p.voter.Serialize(w)
	if err != nil {
		return err
	}
	err = serialization.WriteVarBytes(w, p.signature[:])
	if err != nil {
		return err
	}

	return nil
}

func (p *BlockVote) Deserialize(r io.Reader) error {
	err := p.blockHash.Deserialize(r)
	if err != nil {
		return err
	}
	agree, err := serialization.ReadBool(r)
	if err != nil {
		return err
	}
	p.agree = agree
	err = p.voter.DeSerialize(r)
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

