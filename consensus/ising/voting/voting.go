package voting

import (
	. "github.com/nknorg/nkn/common"
)

type VotingContentType byte

const (
	SigChainTxnVote VotingContentType = 0
	BlockVote       VotingContentType = 1
)

type VotingContent interface {
	Hash() Uint256
}

type Voting interface {
	// add voting entity to cache
	AddToCache(content VotingContent) error
	// get voting content type
	VotingType() VotingContentType
	// set hash in process
	SetConfirmingHash(hash Uint256)
	// get hash in process
	GetConfirmingHash() Uint256
	// set self state
	SetSelfState(hash Uint256, s State)
	// check proposer state
	HasSelfState(hash Uint256, s State) bool
	// set voter state
	SetNeighborState(nid uint64, hash Uint256, s State)
	// check voter state
	HasNeighborState(nid uint64, hash Uint256, s State) bool
	// get current voting height
	GetVotingHeight() uint32
	// get best voting content
	GetBestVotingContent(height uint32) (VotingContent, error)
	// get worse voting content for testing mind changing
	GetWorseVotingContent(height uint32) (VotingContent, error)
	// get voting content from memory pool
	GetVotingContentFromPool(hash Uint256, height uint32) (VotingContent, error)
	// get voting content from memory pool and ledger
	GetVotingContent(hash Uint256, height uint32) (VotingContent, error)
	// get voting pool
	GetVotingPool() VotingPool
	// check if exist in local memory
	Exist(hash Uint256, height uint32) bool
	// dump consensus state for testing
	DumpState(hash Uint256, desc string, verbose bool)
}
