package moca

import (
	"time"

	"github.com/nknorg/nkn/util/config"
)

const (
	maxNumTxnPerBlock           = 20480
	electionStartDelay          = 10 * time.Second
	electionDuration            = 10 * time.Second
	minVotingInterval           = 500 * time.Millisecond
	proposingInterval           = 500 * time.Millisecond
	cacheExpiration             = 3600 * time.Second
	cacheCleanupInterval        = 600 * time.Second
	proposingStartDelay         = config.ProposerChangeTime + time.Second
	getConsensusStateInterval   = 30 * time.Second
	getConsensusStateRetries    = 3
	getConsensusStateRetryDelay = 3 * time.Second
	proposalChanLen             = 100
	requestProposalChanLen      = 10000
	changeVoteMinRelativeWeight = 0.5
	consensusMinRelativeWeight  = 2.0 / 3.0
	syncMinRelativeWeight       = 1.0 / 2.0
)
