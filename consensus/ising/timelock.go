package ising

import (
	"fmt"
	"sync"
	"time"
)

type TimeLock struct {
	sync.RWMutex
	locker map[uint32]*time.Timer
}

func NewTimeLock() *TimeLock {
	return &TimeLock{
		locker: make(map[uint32]*time.Timer),
	}
}

func (tl *TimeLock) LockForHeight(height uint32, timer *time.Timer) {
	tl.Lock()
	defer tl.Unlock()

	if _, ok := tl.locker[height]; !ok {
		tl.locker[height] = timer
	}
}

func (tl *TimeLock) RemoveForHeight(height uint32) {
	tl.Lock()
	defer tl.Unlock()

	delete(tl.locker, height)
}

func (tl *TimeLock) WaitForTimeout(height uint32) error {
	tl.RLock()
	defer tl.RUnlock()

	timer, ok := tl.locker[height]
	if !ok {
		return fmt.Errorf("no time locker for height: %d", height)
	}
	// wait for time locker expired
	select {
	case <-timer.C:
		// reset right away after timer expired
		timer.Reset(0)
	}
	return nil
}
