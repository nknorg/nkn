package ising

import (
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/consensus/ising/voting"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
)

type Response struct {
	hash        *Uint256 // response to which hash
	height      uint32
	contentType voting.VotingContentType
	content     voting.VotingContent
}

func NewResponse(hash *Uint256, height uint32, ctype voting.VotingContentType, content voting.VotingContent) *Response {
	return &Response{
		hash:        hash,
		height:      height,
		contentType: ctype,
		content:     content,
	}
}

func (resp *Response) Serialize(w io.Writer) error {
	_, err := resp.hash.Serialize(w)
	if err != nil {
		return err
	}
	err = serialization.WriteUint32(w, resp.height)
	if err != nil {
		return err
	}
	err = serialization.WriteByte(w, byte(resp.contentType))
	if err != nil {
		return err
	}
	switch resp.contentType {
	case voting.SigChainTxnVote:
		if txn, ok := resp.content.(*transaction.Transaction); ok {
			err = txn.Serialize(w)
			if err != nil {
				return err
			}
		}
	case voting.BlockVote:
		if b, ok := resp.content.(*ledger.Block); ok {
			err = b.Serialize(w)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (resp *Response) Deserialize(r io.Reader) error {
	resp.hash = new(Uint256)
	err := resp.hash.Deserialize(r)
	if err != nil {
		return err
	}
	height, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	resp.height = height
	contentType, err := serialization.ReadByte(r)
	if err != nil {
		return err
	}
	resp.contentType = voting.VotingContentType(contentType)
	switch resp.contentType {
	case voting.SigChainTxnVote:
		txn := new(transaction.Transaction)
		err := txn.Deserialize(r)
		if err != nil {
			return err
		}
		resp.content = txn
	case voting.BlockVote:
		block := new(ledger.Block)
		err := block.Deserialize(r)
		if err != nil {
			return err
		}
		resp.content = block
	}

	return nil
}
