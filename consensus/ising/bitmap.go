package ising

type Bitmap uint32

func (p *Bitmap) SetBit(bit Bitmap) {
	*p |= bit
}

func (p Bitmap) HasBit(bit Bitmap) bool {
	return (p & bit) == bit
}

func (p *Bitmap) ClearBit(bit Bitmap) {
	*p &^= bit
}

const (
	BlockProposer Bitmap = 1 << iota
	BlockVoter
)

const (
	InitialState Bitmap = 1 << iota

	// proposer node get into this state after broadcast block
	// voter node get into this state after received block
	FloodingFinished

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
