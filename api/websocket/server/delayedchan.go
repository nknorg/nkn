package server

import (
	"time"
)

type DelayedChan struct {
	buffer chan *delayedValue
	delay  time.Duration
}

type delayedValue struct {
	value       interface{}
	releaseTime time.Time
}

func NewDelayedChan(size int, delay time.Duration) *DelayedChan {
	buffer := make(chan *delayedValue, size)
	return &DelayedChan{
		buffer: buffer,
		delay:  delay,
	}
}

func (dc *DelayedChan) Push(v interface{}) bool {
	dv := &delayedValue{
		value:       v,
		releaseTime: time.Now().Add(dc.delay),
	}
	select {
	case dc.buffer <- dv:
		return true
	default:
		return false
	}
}

func (dc *DelayedChan) Pop() (interface{}, bool) {
	dv, ok := <-dc.buffer
	if !ok {
		return nil, false
	}
	if dv.releaseTime.After(time.Now()) {
		time.Sleep(time.Until(dv.releaseTime))
	}
	return dv.value, true
}
