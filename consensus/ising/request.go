package ising

import (
	"io"

	. "nkn/common"
	"nkn/crypto"
	"nkn/common/serialization"
)

type BlockRequest struct {
	blockHash *Uint256
	requester *crypto.PubKey
	signature [32]byte
}

func (p *BlockRequest) Serialize(w io.Writer) error {
	_, err := p.blockHash.Serialize(w)
	if err != nil {
		return err
	}
	err = p.requester.Serialize(w)
	if err != nil {
		return err
	}
	err = serialization.WriteVarBytes(w, p.signature[:])
	if err != nil {
		return err
	}

	return nil
}

func (p *BlockRequest) Deserialize(r io.Reader) error {
	err := p.blockHash.Deserialize(r)
	if err != nil {
		return err
	}
	err = p.requester.DeSerialize(r)
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
