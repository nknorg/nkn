package event

import (
	"fmt"
	"sync"
)

type EventFunc func(v interface{})

type EventQueue struct {
	sync.RWMutex
	subscribers map[EventType][]EventFunc
}

var Queue = NewEventQueue()

func NewEventQueue() *EventQueue {
	return &EventQueue{
		subscribers: make(map[EventType][]EventFunc),
	}
}

// Subscribe adds a new subscriber to Event.
func (eq *EventQueue) Subscribe(eventType EventType, eventFunc EventFunc) int {
	eq.Lock()
	defer eq.Unlock()

	eq.subscribers[eventType] = append(eq.subscribers[eventType], eventFunc)

	return len(eq.subscribers[eventType]) - 1
}

// Unsubscribe removes the specified subscriber
func (eq *EventQueue) Unsubscribe(eventType EventType, subscriberIdx int) error {
	eq.Lock()
	defer eq.Unlock()

	if subscriberIdx >= len(eq.subscribers[eventType]) {
		return fmt.Errorf("no subscriber %v", subscriberIdx)
	}

	eq.subscribers[eventType][subscriberIdx] = nil

	return nil
}

// Notify subscribers that Subscribe specified event
func (eq *EventQueue) Notify(eventType EventType, value interface{}) {
	eq.RLock()
	defer eq.RUnlock()

	if subscribers, ok := eq.subscribers[eventType]; ok {
		for _, f := range subscribers {
			if f != nil {
				go f(value)
			}
		}
	}
}
