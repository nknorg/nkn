package por

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/golang/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	nknerrors "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/types"
	"github.com/nknorg/nkn/util/log"
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
		return c[i].CompareTo(c[j]) < 0
	}

	return false
}

func NewPorPackage(txn *types.Transaction) (*PorPackage, error) {
	if txn.UnsignedTx.Payload.Type != types.CommitType {
		return nil, errors.New("Transaction type mismatch")
	}
	payload, err := types.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return nil, err
	}

	rs := payload.(*types.Commit)
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
		err := errors.New("no miner node in signature chain")
		return nil, nknerrors.NewDetailErr(err, nknerrors.ErrNoCode, err.Error())
	}

	blockHash, err := Uint256ParseFromBytes(sigChain.BlockHash)
	if err != nil {
		return nil, err
	}

	if blockHash == EmptyUint256 {
		return nil, fmt.Errorf("block hash in sigchain is empty")
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

func (pp *PorPackage) CompareTo(o *PorPackage) int {
	return bytes.Compare(pp.SigHash, o.SigHash)
}

func (pp *PorPackage) DumpInfo() {
	log.Info("owner: ", BytesToHexString(pp.Owner))
	log.Info("txHash: ", pp.TxHash)
	log.Info("sigHash: ", pp.SigHash)
	sc := pp.SigChain
	sc.DumpInfo()
}
