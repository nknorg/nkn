package node

import (
	"fmt"

	"github.com/nknorg/nkn/events"
)

type eventQueue struct {
	Consensus *events.Event
	Block     *events.Event
	Relay     *events.Event
	Syncing   *events.Event
}

func (eq *eventQueue) init() {
	eq.Consensus = events.NewEvent()
	eq.Block = events.NewEvent()
	eq.Relay = events.NewEvent()
	eq.Syncing = events.NewEvent()
}

func (eq *eventQueue) GetEvent(eventName string) *events.Event {
	switch eventName {
	case "consensus":
		return eq.Consensus
	case "block":
		return eq.Block
	case "relay":
		return eq.Relay
	case "sync":
		return eq.Syncing
	default:
		fmt.Printf("Unknow event")
		return nil
	}
}
