package election

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type electionState uint8

const (
	initialized electionState = 0
	started     electionState = 1
	stopped     electionState = 2
)

// Config is the election config.
type Config struct {
	Duration                    time.Duration
	MinVotingInterval           time.Duration
	ChangeVoteMinRelativeWeight float32
	ChangeVoteMinAbsoluteWeight uint32
	ConsensusMinRelativeWeight  float32
	ConsensusMinAbsoluteWeight  uint32
	GetWeight                   func(interface{}) uint32
}

// Election is the structure of an election.
type Election struct {
	*Config
	startOnce     sync.Once
	neighborVotes sync.Map
	voteReceived  chan struct{}
	txVoteChan    chan interface{}

	sync.RWMutex
	selfVote interface{}
	state    electionState
}

// NewElection creates an election using the config provided.
func NewElection(config *Config) (*Election, error) {
	if config.Duration == 0 {
		return nil, errors.New("Election duration cannot be empty")
	}

	if config.GetWeight == nil {
		config.GetWeight = func(interface{}) uint32 { return 1 }
	}

	election := &Election{
		Config:       config,
		state:        initialized,
		voteReceived: make(chan struct{}, 1),
		txVoteChan:   make(chan interface{}),
	}

	return election, nil
}

// SetInitialVote sets the initial vote if election has not started, otherwise
// returns error.
func (election *Election) SetInitialVote(vote interface{}) error {
	if election.HasStarted() {
		return errors.New("Cannot set initial vote, election has started")
	}

	election.Lock()
	election.selfVote = vote
	election.Unlock()

	return nil
}

// Start starts an election and will stop the election after duration. Returns
// if start success. Multiple concurrent call will only return success once.
func (election *Election) Start() bool {
	success := false

	election.startOnce.Do(func() {
		election.Lock()
		election.state = started
		election.Unlock()

		go election.updateVote()

		time.AfterFunc(election.Duration, func() {
			election.Stop()
		})

		success = true
	})

	return success
}

// Stop stops an election. Typically this should not be called directly.
func (election *Election) Stop() {
	election.Lock()
	election.state = stopped
	election.Unlock()

	select {
	case election.voteReceived <- struct{}{}:
	default:
	}
}

// HasStarted returns if an election has started.
func (election *Election) HasStarted() bool {
	election.RLock()
	defer election.RUnlock()
	return election.state != initialized
}

// IsStopped return if an election is stopped.
func (election *Election) IsStopped() bool {
	election.RLock()
	defer election.RUnlock()
	return election.state == stopped
}

// ReceiveVote receives and saves a vote from a neighbor.
func (election *Election) ReceiveVote(neighborID, vote interface{}) error {
	if election.IsStopped() {
		return errors.New("Election has already stopped")
	}

	election.neighborVotes.Store(neighborID, vote)

	select {
	case election.voteReceived <- struct{}{}:
	default:
	}

	return nil
}

// GetTxVoteChan returns the send vote channel, which should be used to send
// votes to neighbors.
func (election *Election) GetTxVoteChan() <-chan interface{} {
	return election.txVoteChan
}

// GetResult returns the winner vote if the election is stopped, otherwise
// returns error.
func (election *Election) GetResult() (interface{}, error) {
	if !election.IsStopped() {
		return nil, errors.New("election has not stopped yet")
	}

	result, absWeight, relWeight := election.getLeadingVote()
	if absWeight < election.ConsensusMinAbsoluteWeight {
		return nil, fmt.Errorf("leading vote %v only got %d weight, which is less than threshold %d", result, absWeight, election.ConsensusMinAbsoluteWeight)
	}
	if relWeight < election.ConsensusMinRelativeWeight {
		return nil, fmt.Errorf("leading vote %v only got %f%% weight, which is less than threshold %f%%", result, relWeight*100, election.ConsensusMinRelativeWeight*100)
	}
	if result == nil {
		return nil, errors.New("election result is nil")
	}

	return result, nil
}

// GetNeighborIDsByVote get neighbor id list that vote for a certain vote
func (election *Election) GetNeighborIDsByVote(vote interface{}) []interface{} {
	var neighborIDs []interface{}
	election.neighborVotes.Range(func(key, value interface{}) bool {
		if value == vote {
			neighborIDs = append(neighborIDs, key)
		}
		return true
	})
	return neighborIDs
}

// NeighborVoteCount counts the number of neighbor votes received.
func (election *Election) NeighborVoteCount() int {
	count := 0
	election.neighborVotes.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// updateVote updates self vote and write vote into txVoteChan if self vote
// changes with throttle.
func (election *Election) updateVote() {
	time.Sleep(election.MinVotingInterval)

	for {
		<-election.voteReceived

		if election.IsStopped() {
			close(election.txVoteChan)
			return
		}

		election.RLock()
		leadingVote, absWeight, relWeight := election.getLeadingVote()
		selfVote := election.selfVote
		election.RUnlock()

		if absWeight >= election.ChangeVoteMinAbsoluteWeight && relWeight >= election.ChangeVoteMinRelativeWeight {
			if selfVote != leadingVote {
				election.Lock()
				election.selfVote = leadingVote
				election.Unlock()

				election.txVoteChan <- leadingVote

				time.Sleep(election.MinVotingInterval)
			}
		}
	}
}

// getLeadingVote returns the vote with the highest weight, its absolute and
// relative weight.
func (election *Election) getLeadingVote() (interface{}, uint32, float32) {
	weightByVote := make(map[interface{}]uint32)
	if election.selfVote != nil {
		weightByVote[election.selfVote] = election.GetWeight(nil)
	}

	election.neighborVotes.Range(func(key, value interface{}) bool {
		if value != nil {
			weightByVote[value] += election.GetWeight(key)
		}
		return true
	})

	var maxWeight, totalWeight uint32
	var majorityVote interface{}
	for vote, weight := range weightByVote {
		totalWeight += weight
		if weight > maxWeight {
			maxWeight = weight
			majorityVote = vote
		}
	}

	return majorityVote, maxWeight, float32(maxWeight) / float32(totalWeight)
}
