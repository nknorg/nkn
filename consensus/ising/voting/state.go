package voting

const (
	InitialState State = 1 << iota

	// proposer node get into this state after broadcast block
	// voter node get into this state after received block
	FloodingFinished

	// voter node request block from proposer node
	RequestSent

	// proposer node sent the proposal
	ProposalSent

	// voter node sent the idea of the proposal
	OpinionSent
)

type State uint32

func (p *State) SetBit(bit State) {
	*p |= bit
}

func (p State) HasBit(bit State) bool {
	return (p & bit) == bit
}

func (p *State) ClearBit(bit State) {
	*p &^= bit
}

func (p *State) ClearAll() {
	*p = 0
}

