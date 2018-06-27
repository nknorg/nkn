package voting

import (
	"errors"
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/util/log"
)

type BlockVoting struct {
	sync.RWMutex
	pstate         map[Uint256]*State            // consensus state for proposer
	vstate         map[uint64]map[Uint256]*State // consensus state for voter
	height         uint32                        // voting height
	blockCache     *BlockCache                   // received blocks
	pool           *BlockVotingPool              // block voting pool
	confirmingHash Uint256                       // block hash in process
}

func NewBlockVoting(totalWeight int) *BlockVoting {
	blockVoting := &BlockVoting{
		pstate:     make(map[Uint256]*State),
		vstate:     make(map[uint64]map[Uint256]*State),
		height:     ledger.DefaultLedger.Store.GetHeight() + 1,
		blockCache: NewBlockCache(),
		pool:       NewBlockVotingPool(totalWeight),
	}

	return blockVoting
}

func (bv *BlockVoting) SetSelfState(blockhash Uint256, s State) {
	bv.Lock()
	defer bv.Unlock()

	if _, ok := bv.pstate[blockhash]; !ok {
		bv.pstate[blockhash] = new(State)
	}
	bv.pstate[blockhash].SetBit(s)
}

func (bv *BlockVoting) HasSelfState(blockhash Uint256, state State) bool {
	bv.RLock()
	defer bv.RUnlock()

	if v, ok := bv.pstate[blockhash]; !ok || v == nil {
		return false
	} else {
		if v.HasBit(state) {
			return true
		}
		return false
	}
}

func (bv *BlockVoting) SetNeighborState(id uint64, blockhash Uint256, s State) {
	bv.Lock()
	defer bv.Unlock()

	if _, ok := bv.vstate[id]; !ok {
		bv.vstate[id] = make(map[Uint256]*State)
	}
	if _, ok := bv.vstate[id][blockhash]; !ok {
		bv.vstate[id][blockhash] = new(State)
	}
	bv.vstate[id][blockhash].SetBit(s)
}

func (bv *BlockVoting) HasNeighborState(id uint64, blockhash Uint256, state State) bool {
	bv.RLock()
	defer bv.RUnlock()

	if _, ok := bv.vstate[id]; !ok {
		return false
	} else {
		if v, ok := bv.vstate[id][blockhash]; !ok || v == nil {
			return false
		} else {
			if v.HasBit(state) {
				return true
			}
			return false
		}
	}
}

func (bv *BlockVoting) SetVotingHeight(height uint32) {
	bv.height = height
}

func (bv *BlockVoting) GetVotingHeight() uint32 {
	return ledger.DefaultLedger.Store.GetHeight() + 1
}

func (bv *BlockVoting) SetConfirmingHash(hash Uint256) {
	bv.confirmingHash = hash
}

func (bv *BlockVoting) GetConfirmingHash() Uint256 {
	return bv.confirmingHash
}

func (bv *BlockVoting) GetBestVotingContent(height uint32) (VotingContent, error) {
	block := bv.blockCache.GetBestBlockFromCache(height)
	if block == nil {
		return nil, errors.New("no block available")
	}

	return block, nil
}

func (bv *BlockVoting) GetWorseVotingContent(height uint32) (VotingContent, error) {
	block := bv.blockCache.GetWorseBlockFromCache(height)
	if block == nil {
		return nil, errors.New("no block available")
	}

	return block, nil
}

func (bv *BlockVoting) GetVotingContentFromPool(hash Uint256, height uint32) (VotingContent, error) {
	block := bv.blockCache.GetBlockFromCache(hash, height)
	if block != nil {
		return block, nil
	}

	return nil, errors.New("invalid hash and height for block")
}

func (bv *BlockVoting) GetVotingContent(hash Uint256, height uint32) (VotingContent, error) {
	// get block from cache
	block, err := bv.GetVotingContentFromPool(hash, height)
	if err == nil {
		return block, nil
	}
	// get block from ledger
	block, err = ledger.DefaultLedger.Store.GetBlock(hash)
	if err == nil {
		return block, nil
	}

	return nil, errors.New("invalid hash for block")
}

func (bv *BlockVoting) VotingType() VotingContentType {
	return BlockVote
}

func (bv *BlockVoting) AddToCache(content VotingContent) error {
	var err error
	if block, ok := content.(*ledger.Block); !ok {
		return errors.New("invalid voting content type")
	} else {
		blockHeight := block.Header.Height
		localHeight := ledger.DefaultLedger.Store.GetHeight()
		if blockHeight != localHeight+1 {
			return errors.New("invalid block height")
		}
		err = ledger.BlockFullyCheck(block, ledger.DefaultLedger)
		if err != nil {
			return err
		}
		err = bv.blockCache.AddBlockToCache(block)
		if err != nil {
			return err
		}
	}

	return nil
}

func (bv *BlockVoting) Exist(hash Uint256, height uint32) bool {
	return bv.blockCache.BlockInCache(hash, height)
}

func (bv *BlockVoting) GetVotingPool() VotingPool {
	return bv.pool
}

func (bv *BlockVoting) DumpState(hash Uint256, desc string, verbose bool) {
	str := ""
	s := bv.pstate[hash]
	if s.HasBit(FloodingFinished) {
		str += "FloodingFinished"
	}
	if s.HasBit(RequestSent) {
		str += " -> RequestSent"
	}
	if s.HasBit(ProposalReceived) {
		str += " -> ProposalReceived"
	}
	h := BytesToHexString(hash.ToArray())
	if !verbose {
		h = h[:4]
	}
	log.Infof("BlockHash: %s State: %s | %s", h, str, desc)
}
