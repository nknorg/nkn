package ising

const (
	// proposer node get into this state after broadcast block
	// voter node get into this state after received block
	FloodingFinished = 1 << iota

	// proposer node sent the proposal
	ProposalSent

	// voter node received proposal
	ProposalReceived

	// voter node sent the idea of the proposal
	OpinionSent

	// proposer node received the idea from voter node
	OpintionReceived

	// proposer got enough votes
	BlockConfirmed

	// proposer got enough votes is enough
	BlockDroped
)
