package event

type EventType uint8

const (
	// BlockPersistCompleted is called for both consensus and block syncing
	BlockPersistCompleted EventType = iota
	// NewBlockProduced is called only for consensus, but NOT block syncing
	NewBlockProduced
	SendInboundMessageToClient
	BacktrackSigChain
)
