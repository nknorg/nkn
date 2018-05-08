package ising

import (
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/consensus/ising/voting"
)

type Request struct {
	hash        *Uint256
	contentType voting.VotingContentType
}

func NewRequest(hash *Uint256, ctype voting.VotingContentType) *Request {
	return &Request{
		hash: hash,
		contentType: ctype,
	}
}

func (req *Request) Serialize(w io.Writer) error {
	_, err := req.hash.Serialize(w)
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
	contentType, err := serialization.ReadByte(r)
	if err != nil {
		req.contentType = voting.VotingContentType(contentType)
	}

	return nil
}
