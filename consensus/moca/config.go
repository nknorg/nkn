package moca

import (
	"time"

	"github.com/nknorg/nkn/util/config"
)

const (
	electionStartDelay          = config.ConsensusDuration / 2
	electionDuration            = config.ConsensusDuration / 2
	minVotingInterval           = 500 * time.Millisecond
	proposingInterval           = 500 * time.Millisecond
	cacheExpiration             = 3600 * time.Second
	cacheCleanupInterval        = 600 * time.Second
	proposingStartDelay         = config.ConsensusTimeout + time.Second
	getConsensusStateInterval   = 30 * time.Second
	getConsensusStateRetries    = 3
	getConsensusStateRetryDelay = 3 * time.Second
	proposalChanLen             = 100
	requestProposalChanLen      = 10000
	changeVoteMinRelativeWeight = 0.5
	consensusMinRelativeWeight  = 2.0 / 3.0
	syncMinRelativeWeight       = 1.0 / 2.0
)
