package ising

const (
	// proposer node sent the block
	BlockSent = 0

	// voter node received proposed block
	BlockReceived = 1

	// voter node sent the idea of the proposed block
	OpinionSent = 2

	// proposer node received the idea from voter node
	OpintionReceived = 3

	// proposer got enough votes
	BlockConfirmed = 4

	// proposer got enough votes is enough
	BlockDroped = 5
)
