package ising

import (
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/consensus/ising/voting"
)

type Vote struct {
	hash        *Uint256
	height      uint32
	contentType voting.VotingContentType
	agree       bool
	preferHash  *Uint256
}

func NewVoting(hash *Uint256, height uint32, contentType voting.VotingContentType, agree bool, preferHash *Uint256) *Vote {
	return &Vote{
		hash:        hash,
		height:      height,
		contentType: contentType,
		agree:       agree,
		preferHash:  preferHash,
	}
}

func (v *Vote) Serialize(w io.Writer) error {
	_, err := v.hash.Serialize(w)
	if err != nil {
		return err
	}
	err = serialization.WriteUint32(w, v.height)
	if err != nil {
		return err
	}
	err = serialization.WriteByte(w, byte(v.contentType))
	if err != nil {
		return err
	}
	err = serialization.WriteBool(w, v.agree)
	if err != nil {
		return err
	}
	if !v.agree {
		_, err := v.preferHash.Serialize(w)
		if err != nil {
			return err
		}
	}

	return nil
}

func (v *Vote) Deserialize(r io.Reader) error {
	v.hash = new(Uint256)
	err := v.hash.Deserialize(r)
	if err != nil {
		return err
	}
	height, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	v.height = height
	contentType, err := serialization.ReadByte(r)
	if err != nil {
		return err
	}
	v.contentType = voting.VotingContentType(contentType)
	agree, err := serialization.ReadBool(r)
	if err != nil {
		return err
	}
	v.agree = agree
	if !agree {
		v.preferHash = new(Uint256)
		err = v.preferHash.Deserialize(r)
		if err != nil {
			return err
		}
	}

	return nil
}
