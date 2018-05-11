package voting

import (
	. "github.com/nknorg/nkn/common"
)

type VotingPool interface {
	// set mind for local node
	SetMind(height uint32, hash Uint256)
	// get mind of local node
	GetMind(height uint32) Uint256
	// change mind for local node
	ChangeMind(height uint32, hash Uint256)
	// add a received vote to receive pool
	AddToReceivePool(nid uint64, height uint32, hash Uint256)
	// add a sent vote to send pool
	AddToSendPool(nid uint64, height uint32, hash Uint256)
	// vote counting
	VoteCounting(height uint32) (*Uint256, error)
	// check if need to change mind
	NeedChangeMind(height uint32, hash Uint256) bool
	// if mind changed then notice neighbors who received vote before
	SpreadNewMind()
	// voting pool reset
	Reset(weight int)
}
