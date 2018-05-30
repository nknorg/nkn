package ising

import (
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/consensus/ising/voting"
)

type MindChanging struct {
	hash        *Uint256
	height      uint32
	contentType voting.VotingContentType
}

func NewMindChanging(hash *Uint256, height uint32, ctype voting.VotingContentType) *MindChanging {
	return &MindChanging{
		hash:        hash,
		height:      height,
		contentType: ctype,
	}
}

func (mc *MindChanging) Serialize(w io.Writer) error {
	_, err := mc.hash.Serialize(w)
	if err != nil {
		return err
	}
	err = serialization.WriteUint32(w, mc.height)
	if err != nil {
		return err
	}
	err = serialization.WriteByte(w, byte(mc.contentType))
	if err != nil {
		return err
	}

	return nil
}

func (mc *MindChanging) Deserialize(r io.Reader) error {
	mc.hash = new(Uint256)
	err := mc.hash.Deserialize(r)
	if err != nil {
		return err
	}
	height, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	mc.height = height
	contentType, err := serialization.ReadByte(r)
	if err != nil {
		return err
	}
	mc.contentType = voting.VotingContentType(contentType)

	return nil
}
