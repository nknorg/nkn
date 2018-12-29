package election

import (
	"errors"
	"sync"
	"time"
)

type electionState uint8

const (
	initialized electionState = 0
	started     electionState = 1
	stopped     electionState = 2
)

// Election is the structure of an election.
type Election struct {
	sync.RWMutex
	state             electionState
	startOnce         sync.Once
	duration          time.Duration
	minVotingInterval time.Duration
	getWeight         func(interface{}) uint32
	neighborVotes     sync.Map
	selfVote          interface{}
	voteReceived      chan struct{}
	txVoteChan        chan interface{}
}

// Config is the election config.
type Config struct {
	Duration          time.Duration
	MinVotingInterval time.Duration
	GetWeight         func(interface{}) uint32
}

// NewElection creates an election using the config provided.
func NewElection(config *Config) (*Election, error) {
	if config.Duration == 0 {
		return nil, errors.New("Election duration cannot be empty")
	}

	getWeight := config.GetWeight
	if getWeight == nil {
		getWeight = func(interface{}) uint32 { return 1 }
	}

	election := &Election{
		state:             initialized,
		duration:          config.Duration,
		minVotingInterval: config.MinVotingInterval,
		getWeight:         getWeight,
		voteReceived:      make(chan struct{}, 1),
		txVoteChan:        make(chan interface{}),
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

		time.AfterFunc(election.duration, func() {
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
	election.selfVote = election.getMajorityVote()
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
	if election.selfVote == nil {
		return nil, errors.New("cannot reach consensus")
	}
	return election.selfVote, nil
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
	for {
		<-election.voteReceived

		if election.IsStopped() {
			close(election.txVoteChan)
			return
		}

		election.RLock()
		majorityVote := election.getMajorityVote()
		selfVote := election.selfVote
		election.RUnlock()

		if selfVote != majorityVote {
			election.Lock()
			election.selfVote = majorityVote
			election.Unlock()

			election.txVoteChan <- majorityVote

			time.Sleep(election.minVotingInterval)
		}
	}
}

// getMajorityVote returns the majority of the current voting results.
func (election *Election) getMajorityVote() interface{} {
	votes := make([]interface{}, 0)
	weights := make([]uint32, 0)

	if election.selfVote != nil {
		votes = append(votes, election.selfVote)
		weights = append(weights, election.getWeight(nil))
	}

	election.neighborVotes.Range(func(key, value interface{}) bool {
		found := false
		for i, vote := range votes {
			if vote == value {
				weights[i] += election.getWeight(key)
				found = true
				break
			}
		}

		if !found {
			votes = append(votes, value)
			weights = append(weights, election.getWeight(key))
		}

		return true
	})

	var maxWeight uint32
	var majorityVote interface{}
	for i, weight := range weights {
		if weight > maxWeight {
			maxWeight = weight
			majorityVote = votes[i]
		}
	}

	return majorityVote
}
