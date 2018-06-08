package voting

const (
	InitialState State = 1 << iota

	// set self state after receive block
	FloodingFinished

	// set neighbor state after sending request
	RequestSent

	// set neighbor state after receive request
	RequestReceived

	// set self state after sending proposal
	ProposalSent

	// set neighbor state after receive proposal
	ProposalReceived
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
