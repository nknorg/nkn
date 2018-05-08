package ising

import (
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type Voting struct {
	hash  *Uint256
	agree bool
}

func NewVoting(hash *Uint256, agree bool) *Voting {
	return &Voting{
		hash: hash,
		agree: agree,
	}
}


func (v *Voting) Serialize(w io.Writer) error {
	_, err := v.hash.Serialize(w)
	if err != nil {
		return err
	}
	err = serialization.WriteBool(w, v.agree)
	if err != nil {
		return err
	}

	return nil
}

func (v *Voting) Deserialize(r io.Reader) error {
	v.hash = new(Uint256)
	err := v.hash.Deserialize(r)
	if err != nil {
		return err
	}
	agree, err := serialization.ReadBool(r)
	if err != nil {
		return err
	}
	v.agree = agree

	return nil
}

