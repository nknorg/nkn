package node

type ChanQueue chan struct{}

func MakeChanQueue(n int) ChanQueue {
	return make(chan struct{}, n)
}

func (s ChanQueue) acquire() { s <- struct{}{} }
func (s ChanQueue) release() { <-s }
