package consensus

import (
	"time"

	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/pb"
)

const (
	electionStartDelay            = config.ConsensusDuration / 2
	electionDuration              = config.ConsensusDuration / 2
	proposalVerificationTimeout   = electionStartDelay * 4 / 5
	initialVoteDelay              = electionStartDelay / 2
	minVotingInterval             = 200 * time.Millisecond
	maxVotingInterval             = 2 * time.Second
	proposingInterval             = 500 * time.Millisecond
	proposingTimeout              = chain.ProposingTimeTolerance * 4 / 5
	cacheExpiration               = 3600 * time.Second
	cacheCleanupInterval          = 600 * time.Second
	proposingStartDelay           = 4*config.ConsensusTimeout + time.Second
	proposalPropagationDelay      = time.Second
	getConsensusStateInterval     = config.ConsensusDuration / 4
	getConsensusStateRetries      = 3
	getConsensusStateRetryDelay   = 3 * time.Second
	proposalChanLen               = 100
	requestProposalChanLen        = 10000
	changeVoteMinRelativeWeight   = 0.5
	consensusMinRelativeWeight    = 2.0 / 3.0
	syncMinRelativeWeight         = 1.0 / 2.0
	defaultRequestTransactionType = pb.RequestTransactionType_REQUEST_TRANSACTION_SHORT_HASH
	acceptVoteHeightRange         = 32
	minConsensusStateNeighbors    = 16
)
