package voting

import (
	"sync"

	. "github.com/nknorg/nkn/common"
)

type SigChainVoting struct {
	sync.RWMutex
	pstate         map[Uint256]*State            // consensus state for proposer
	vstate         map[uint64]map[Uint256]*State // consensus state for voter
	confirmingHash Uint256                       // sigchain hash in process
}

func NewSigChainVoting() *SigChainVoting {
	sigchainVoting := &SigChainVoting{
		pstate: make(map[Uint256]*State),
		vstate: make(map[uint64]map[Uint256]*State),
	}

	return sigchainVoting
}

func (scv *SigChainVoting) SetProposerState(blockhash Uint256, s State) {
	scv.Lock()
	defer scv.Unlock()

	if _, ok := scv.pstate[blockhash]; !ok {
		scv.pstate[blockhash] = new(State)
	}
	scv.pstate[blockhash].SetBit(s)
}

func (scv *SigChainVoting) HasProposerState(blockhash Uint256, state State) bool {
	scv.RLock()
	defer scv.RUnlock()

	if v, ok := scv.pstate[blockhash]; !ok || v == nil {
		return false
	} else {
		if v.HasBit(state) {
			return true
		}
		return false
	}
}

func (scv *SigChainVoting) SetVoterState(id uint64, blockhash Uint256, s State) {
	if _, ok := scv.vstate[id]; !ok {
		scv.vstate[id] = make(map[Uint256]*State)
	}
	if _, ok := scv.vstate[id][blockhash]; !ok {
		scv.vstate[id][blockhash] = new(State)
	}
	scv.vstate[id][blockhash].SetBit(s)
}

func (scv *SigChainVoting) HasVoterState(id uint64, blockhash Uint256, state State) bool {
	if _, ok := scv.vstate[id]; !ok {
		return false
	} else {
		if v, ok := scv.vstate[id][blockhash]; !ok || v == nil {
			return false
		} else {
			if v.HasBit(state) {
				return true
			}
			return false
		}
	}
}

func (scv *SigChainVoting) SetConfirmingHash(hash Uint256) {
	scv.confirmingHash = hash
}

func (scv *SigChainVoting) GetConfirmingHash() Uint256 {
	return scv.confirmingHash
}

func (scv *SigChainVoting) GetCurrentVotingContent() (VotingContent, error) {
	return nil, nil
}

func (scv *SigChainVoting) GetVotingContent(hash Uint256) (VotingContent, error) {
	return nil, nil
}

func (scv *SigChainVoting) VotingType() VotingContentType {
	return SigChainVote
}

func (scv *SigChainVoting) Preparing(content VotingContent) error {
	return nil
}

func (scv *SigChainVoting) Exist(hash Uint256) bool {
	return false
}

func (scv *SigChainVoting) DumpState(hash Uint256, desc string, verbose bool) {
}
