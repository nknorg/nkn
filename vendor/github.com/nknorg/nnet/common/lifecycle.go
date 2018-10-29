package common

import "sync"

// LifeCycle is an abstract type that has thread-safe initialization and shutdown
type LifeCycle struct {
	StartOnce sync.Once
	StopOnce  sync.Once
	stopped   bool
	stopLock  sync.RWMutex
}

// Stop set lifecycle status to stopped
func (l *LifeCycle) Stop() {
	l.stopLock.Lock()
	l.stopped = true
	l.stopLock.Unlock()
}

// IsStopped returns if lifecycle is stopped
func (l *LifeCycle) IsStopped() bool {
	l.stopLock.RLock()
	defer l.stopLock.RUnlock()
	return l.stopped
}
