package voting

import (
	. "github.com/nknorg/nkn/common"
)

const (
	MinVoterNum = 3
)

type VoteInfo struct {
	votedHash   Uint256
	voterWeight int
}

type VotingPool interface {
	// get mind of local node
	GetMind(height uint32) (Uint256, bool)
	// change mind for local node
	SetMind(height uint32, hash Uint256) map[uint64]Uint256
	// add a sent vote to send pool
	AddToSendPool(height uint32, nid uint64, hash Uint256)
	// add a received vote to receive pool
	AddToReceivePool(height uint32, nid uint64, weight int, hash Uint256)
	// check if has received vote from a specified neighbor
	HasReceivedVoteFrom(height uint32, nodeID uint64) bool
	// add vote to receive pool then count votes
	AddVoteThenCounting(height uint32, nodeID uint64, weight int, hash Uint256) (*Uint256, error)
	// voting pool reset
	Reset()
}
