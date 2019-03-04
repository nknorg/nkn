package por

import (
	"bytes"
	"errors"

	"github.com/golang/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	. "github.com/nknorg/nkn/pb"
	. "github.com/nknorg/nkn/transaction"
)

const (
	// The height of signature chain which run for block proposer should be (local block height -1 + 5)
	// -1 means that:
	//  local block height may heigher than neighbor node at most 1
	// +5 means that:
	// if local block height is n, then n + 3 signature chain is in consensus) +
	//  1 (since local node height may lower than neighbors at most 1) +
	//  1 (for fully propagate)
	SigChainBlockHeightOffset  = 1
	SigChainMiningHeightOffset = 4
)

type PorStore interface {
	GetHeightByBlockHash(hash Uint256) (uint32, error)
}

var Store PorStore

type PorPackages []*PorPackage

func (c PorPackages) Len() int {
	return len(c)
}

func (c PorPackages) Swap(i, j int) {
	if i >= 0 && i < len(c) && j >= 0 && j < len(c) { // Unit Test modify
		c[i], c[j] = c[j], c[i]
	}
}

func (c PorPackages) Less(i, j int) bool {
	if i >= 0 && i < len(c) && j >= 0 && j < len(c) { // Unit Test modify
		return bytes.Compare(c[i].SigHash, c[j].SigHash) < 0
	}

	return false
}

func NewPorPackage(txn *Transaction) (*PorPackage, error) {
	if txn.UnsignedTx.Payload.Type != CommitType {
		return nil, errors.New("Transaction type mismatch")
	}
	payload, err := Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return nil, err
	}

	rs := payload.(*Commit)
	sigChain := &SigChain{}
	err = proto.Unmarshal(rs.SigChain, sigChain)
	if err != nil {
		return nil, err
	}

	err = sigChain.Verify()
	if err != nil {
		return nil, err
	}

	err = sigChain.VerifyPath()
	if err != nil {
		return nil, err
	}

	//TODO threshold
	found := false
	for _, elem := range sigChain.Elems {
		if elem.Mining == true {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("No miner node in signature chain")
	}

	blockHash, err := Uint256ParseFromBytes(sigChain.BlockHash)
	if err != nil {
		return nil, err
	}

	if blockHash == EmptyUint256 {
		return nil, errors.New("block hash in sigchain is empty")
	}

	height, err := Store.GetHeightByBlockHash(blockHash)
	if err != nil {
		return nil, err
	}

	owner, err := sigChain.GetOwner()
	if err != nil {
		return nil, err
	}

	txHash := txn.Hash()
	sigHash, err := sigChain.SignatureHash()
	if err != nil {
		return nil, err
	}
	pp := &PorPackage{
		VoteForHeight: height + SigChainMiningHeightOffset + SigChainBlockHeightOffset,
		Owner:         owner,
		BlockHash:     sigChain.BlockHash,
		TxHash:        txHash[:],
		SigHash:       sigHash,
		SigChain:      sigChain,
	}
	return pp, nil
}
