package chord

import (
	"bytes"
	"sort"

	"github.com/nknorg/nkn/util/log"
)

func (r *Ring) DumpInfo(finger bool) {
	for idx, vnode := range r.vnodes {
		log.Infof("ring.vnodes[%d]: {", idx)
		log.Infof("\tId: %x", string(vnode.Id))
		log.Infof("\tHost: %s", vnode.Host)

		for sidx, succ := range vnode.successors {
			if succ != nil {
				log.Infof("\tsucc[%d].Id: %x", sidx, string(succ.Id))
				log.Infof("\tsucc[%d].Host: %s", sidx, succ.Host)
			} else {
				log.Infof("\tsucc[%d]: nil", sidx)
			}
		}

		// Only dump []finger when DumpInfo(true)
		if finger {
			for fidx, fing := range vnode.finger {
				if fing != nil {
					log.Infof("\tfinger[%d].Id: %x", fidx, string(fing.Id))
					log.Infof("\tfinger[%d].Host: %s", fidx, fing.Host)
				} else {
					log.Infof("\tfinger[%d]: nil", fidx)
				}
			}
		}

		log.Infof("\tlast_finger: %d", vnode.last_finger)
		if vnode.predecessor != nil {
			log.Infof("\tpredecessor.Id: %x", string((*vnode.predecessor).Id))
			log.Infof("\tpredecessor.Host: %s", (*vnode.predecessor).Host)
		}
		log.Infof("\tstabilized: %s", vnode.stabilized.String())
		log.Infof("}\n")
	}
}

func (r *Ring) init(conf *Config, trans Transport) {
	// Set our variables
	r.config = conf
	r.vnodes = make([]*localVnode, conf.NumVnodes)
	r.transport = InitLocalTransport(trans)
	r.delegateCh = make(chan func(), 32)

	// Initializes the vnodes
	for i := 0; i < conf.NumVnodes; i++ {
		vn := &localVnode{}
		r.vnodes[i] = vn
		vn.ring = r
		vn.init(i)
	}

	// Sort the vnodes
	sort.Sort(r)
}

// Len is the number of vnodes
func (r *Ring) Len() int {
	return len(r.vnodes)
}

// Less returns whether the vnode with index i should sort
// before the vnode with index j.
func (r *Ring) Less(i, j int) bool {
	return bytes.Compare(r.vnodes[i].Id, r.vnodes[j].Id) == -1
}

// Swap swaps the vnodes with indexes i and j.
func (r *Ring) Swap(i, j int) {
	r.vnodes[i], r.vnodes[j] = r.vnodes[j], r.vnodes[i]
}

// Returns the nearest local vnode to the key
func (r *Ring) nearestVnode(key []byte) *localVnode {
	for i := len(r.vnodes) - 1; i >= 0; i-- {
		if bytes.Compare(r.vnodes[i].Id, key) == -1 {
			return r.vnodes[i]
		}
	}
	// Return the last vnode
	return r.vnodes[len(r.vnodes)-1]
}

// Schedules each vnode in the ring
func (r *Ring) schedule() {
	if r.config.Delegate != nil {
		go r.delegateHandler()
	}
	for i := 0; i < len(r.vnodes); i++ {
		r.vnodes[i].schedule()
	}
}

// Wait for all the vnodes to shutdown
func (r *Ring) stopVnodes() {
	r.shutdown = make(chan bool, r.config.NumVnodes)
	for i := 0; i < r.config.NumVnodes; i++ {
		<-r.shutdown
	}
}

// Stops the delegate handler
func (r *Ring) stopDelegate() {
	if r.config.Delegate != nil {
		// Wait for all delegate messages to be processed
		<-r.invokeDelegate(r.config.Delegate.Shutdown)
		close(r.delegateCh)
	}
}

// Initializes the vnodes with their local successors
func (r *Ring) setLocalSuccessors() {
	numV := len(r.vnodes)
	numSuc := min(r.config.NumSuccessors, numV-1)
	for idx, vnode := range r.vnodes {
		for i := 0; i < numSuc; i++ {
			vnode.successors[i] = &r.vnodes[(idx+i+1)%numV].Vnode
		}
	}
}

// Invokes a function on the delegate and returns completion channel
func (r *Ring) invokeDelegate(f func()) chan struct{} {
	if r.config.Delegate == nil {
		return nil
	}

	ch := make(chan struct{}, 1)
	wrapper := func() {
		defer func() {
			ch <- struct{}{}
		}()
		f()
	}

	r.delegateCh <- wrapper
	return ch
}

// This handler runs in a go routine to invoke methods on the delegate
func (r *Ring) delegateHandler() {
	for {
		f, ok := <-r.delegateCh
		if !ok {
			break
		}
		r.safeInvoke(f)
	}
}

// Called to safely call a function on the delegate
func (r *Ring) safeInvoke(f func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Fatal("Caught a panic invoking a delegate function! Got: %s", r)
		}
	}()
	f()
}

// Len is the number of vnodes
func (r *Ring) GetFirstVnode() *localVnode {
	if r == nil || len(r.vnodes) == 0 {
		return nil
	}
	return r.vnodes[0]
}
