package events

type EventType int16

const (
	EventBlockPersistCompleted EventType = 0
	EventConsensusMsgReceived  EventType = 3
	EventRelayMsgReceived      EventType = 4
	EventBlockSyncingFinished  EventType = 5
)
