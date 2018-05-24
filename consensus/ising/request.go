package ising

import (
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/consensus/ising/voting"
)

type Request struct {
	hash        *Uint256
	height      uint32
	contentType voting.VotingContentType
}

func NewRequest(hash *Uint256, height uint32, ctype voting.VotingContentType) *Request {
	return &Request{
		hash:        hash,
		height:      height,
		contentType: ctype,
	}
}

func (req *Request) Serialize(w io.Writer) error {
	_, err := req.hash.Serialize(w)
	if err != nil {
		return err
	}
	err = serialization.WriteUint32(w, req.height)
	if err != nil {
		return err
	}
	err = serialization.WriteByte(w, byte(req.contentType))
	if err != nil {
		return err
	}

	return nil
}

func (req *Request) Deserialize(r io.Reader) error {
	req.hash = new(Uint256)
	err := req.hash.Deserialize(r)
	if err != nil {
		return err
	}
	height, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	req.height = height
	contentType, err := serialization.ReadByte(r)
	if err != nil {
		return err
	}
	req.contentType = voting.VotingContentType(contentType)

	return nil
}
