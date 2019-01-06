package events

type EventType int16

const (
	EventBlockPersistCompleted       EventType = 0
	EventSendInboundMessageToClient  EventType = 1
	EventReceiveClientSignedSigChain EventType = 2
	EventRelayMsgReceived            EventType = 4
	EventBlockSyncingFinished        EventType = 5
)
