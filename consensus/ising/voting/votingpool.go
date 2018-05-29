package voting

import (
	. "github.com/nknorg/nkn/common"
)

type VotingPool interface {
	// get mind of local node
	GetMind(height uint32) (Uint256, bool)
	// change mind for local node
	ChangeMind(height uint32, hash Uint256) map[uint64]Uint256
	// add a received vote to receive pool
	AddToReceivePool(height uint32, nid uint64, hash Uint256)
	// get receive pool by height
	GetReceivePool(height uint32) map[uint64]Uint256
	// add a sent vote to send pool
	AddToSendPool(height uint32, nid uint64, hash Uint256)
	// vote counting
	VoteCounting(height uint32) (*Uint256, error)
	// voting pool reset
	Reset(weight int)
}
