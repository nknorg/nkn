package por

import (
	"bytes"
	"errors"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/util/log"
)

const (
	// The height of signature chain which run for block proposer should be (local block height -1 + 4)
	// -1 means that:
	//  local block height may heigher than neighbor node at most 1
	// +4 means that:
	//  2 (if local block height is n, then n + 2 signature chain is in consensus) +
	//  1 (since local node height may lower than neighbors at most 1) +
	//  1 (for fully propagate)
	HeightThreshold = 4
)

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

func NewPorPackage(txn *transaction.Transaction) (*PorPackage, error) {
	if txn.TxType != transaction.Commit {
		return nil, errors.New("Transaction type mismatch")
	}
	rs := txn.Payload.(*payload.Commit)
	sigChain := &SigChain{}
	err := proto.Unmarshal(rs.SigChain, sigChain)
	if err != nil {
		return nil, err
	}

	//TODO threshold

	blockHash, err := common.Uint256ParseFromBytes(sigChain.BlockHash)
	if err != nil {
		log.Error("Parse block hash uint256 from bytes error:", err)
		return nil, err
	}
	blockHeader, err := ledger.DefaultLedger.Store.GetHeader(blockHash)
	if err != nil {
		log.Error("Get block header error:", err)
		return nil, err
	}

	owner, err := sigChain.GetOwner()
	if err != nil {
		log.Error("Get owner error:", err)
		return nil, err
	}

	txHash := txn.Hash()
	sigHash, err := sigChain.SignatureHash()
	if err != nil {
		return nil, err
	}
	pp := &PorPackage{
		VoteForHeight: blockHeader.Height + HeightThreshold,
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
	log.Info("owner: ", common.BytesToHexString(pp.Owner))
	log.Info("txHash: ", pp.TxHash)
	log.Info("sigHash: ", pp.SigHash)
	sc := pp.SigChain
	sc.DumpInfo()
}
