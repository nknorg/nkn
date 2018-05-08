package ising

import (
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/consensus/ising/voting"
	"github.com/nknorg/nkn/common/serialization"
)

type Proposal struct {
	hash        *Uint256
	contentType voting.VotingContentType
}

func NewProposal(hash *Uint256, ctype voting.VotingContentType) *Proposal {
	return &Proposal{
		hash: hash,
		contentType: ctype,
	}
}

func (p *Proposal) Serialize(w io.Writer) error {
	_, err := p.hash.Serialize(w)
	if err != nil {
		return err
	}
	err = serialization.WriteByte(w, byte(p.contentType))
	if err != nil {
		return err
	}

	return nil
}

func (p *Proposal) Deserialize(r io.Reader) error {
	p.hash = new(Uint256)
	err := p.hash.Deserialize(r)
	if err != nil {
		return err
	}
	contentType, err := serialization.ReadByte(r)
	if err != nil {
		p.contentType = voting.VotingContentType(contentType)
	}

	return nil
}
