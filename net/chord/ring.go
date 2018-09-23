package chord

import (
	"errors"
	"math/big"
	"sort"

	"github.com/nknorg/nkn/util/log"
)

func (r *Ring) DumpInfo(finger bool) {
	for idx, vnode := range r.Vnodes {
		log.Debugf("ring.Vnodes[%d]: {", idx)
		log.Debugf("\tId: %x", string(vnode.Id))
		log.Debugf("\tHost: %s", vnode.Host)

		for sidx, succ := range vnode.successors {
			if succ != nil {
				log.Debugf("\tsucc[%d].Id: %x", sidx, string(succ.Id))
				log.Debugf("\tsucc[%d].Host: %s", sidx, succ.Host)
			} else {
				log.Debugf("\tsucc[%d]: nil", sidx)
			}
		}

		// Only dump []finger when DumpInfo(true)
		if finger {
			for fidx, fing := range vnode.finger {
				if fing != nil {
					log.Debugf("\tfinger[%d].Id: %x", fidx, string(fing.Id))
					log.Debugf("\tfinger[%d].Host: %s", fidx, fing.Host)
				} else {
					log.Debugf("\tfinger[%d]: nil", fidx)
				}
			}
		}

		log.Debugf("\tlast_finger: %d", vnode.last_finger)
		if vnode.predecessor != nil {
			log.Debugf("\tpredecessor.Id: %x", string((*vnode.predecessor).Id))
			log.Debugf("\tpredecessor.Host: %s", (*vnode.predecessor).Host)
		}
		log.Debugf("\tstabilized: %s", vnode.stabilized.String())
		log.Debugf("}\n")
	}
}

func (r *Ring) init(conf *Config, trans Transport) {
	// Set our variables
	r.config = conf
	r.Vnodes = make([]*localVnode, conf.NumVnodes)
	r.transport = InitLocalTransport(trans)
	r.delegateCh = make(chan func(), 32)

	// Initializes the vnodes
	for i := 0; i < conf.NumVnodes; i++ {
		vn := &localVnode{}
		r.Vnodes[i] = vn
		vn.ring = r
		vn.init(i)
	}

	// Sort the vnodes
	sort.Sort(r)
}

// Len is the number of vnodes
func (r *Ring) Len() int {
	return len(r.Vnodes)
}

// Less returns whether the vnode with index i should sort
// before the vnode with index j.
func (r *Ring) Less(i, j int) bool {
	return CompareId(r.Vnodes[i].Id, r.Vnodes[j].Id) == -1
}

// Swap swaps the vnodes with indexes i and j.
func (r *Ring) Swap(i, j int) {
	r.Vnodes[i], r.Vnodes[j] = r.Vnodes[j], r.Vnodes[i]
}

// Returns the nearest local vnode to the key
func (r *Ring) nearestVnode(key []byte) *localVnode {
	for i := len(r.Vnodes) - 1; i >= 0; i-- {
		if CompareId(r.Vnodes[i].Id, key) == -1 {
			return r.Vnodes[i]
		}
	}
	// Return the last vnode
	return r.Vnodes[len(r.Vnodes)-1]
}

// Schedules each vnode in the ring
func (r *Ring) schedule() {
	if r.config.Delegate != nil {
		go r.delegateHandler()
	}
	for i := 0; i < len(r.Vnodes); i++ {
		r.Vnodes[i].schedule()
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
	numV := len(r.Vnodes)
	numSuc := min(r.config.NumSuccessors, numV-1)
	for idx, vnode := range r.Vnodes {
		for i := 0; i < numSuc; i++ {
			vnode.successors[i] = &r.Vnodes[(idx+i+1)%numV].Vnode
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
			log.Errorf("Caught a panic invoking a delegate function! Got: %s", r)
		}
	}()
	f()
}

func (r *Ring) GetFirstVnode() (*localVnode, error) {
	if r == nil {
		return nil, errors.New("Ring is empty")
	}
	if len(r.Vnodes) == 0 {
		return nil, errors.New("No vnode available")
	}
	return r.Vnodes[0], nil
}

func (r *Ring) GetPredecessor(key []byte) (*Vnode, error) {
	vnode, err := r.GetFirstVnode()
	if err != nil {
		return nil, err
	}
	if vnode == nil {
		return nil, errors.New("No vnode in ring")
	}
	return vnode.FindPredecessor(key)
}

func (r *Ring) shouldConnectToHost(host string) bool {
	for _, vn := range r.Vnodes {
		if vn != nil && vn.shouldConnectToHost(host) {
			return true
		}
	}
	return false
}

// ToData: Extract marshalable data from Ring struct
func (r *Ring) ToData() *RingData {
	if r == nil {
		return nil
	}

	c := r.config.toData()
	nodes := make([]*localVnodeData, len(r.Vnodes))

	for i, n := range r.Vnodes {
		nodes[i] = n.toData()
	}

	return &RingData{Conf: c, Vnodes: nodes}
}

func (r *Ring) Distance(fromId, toId []byte) *big.Int {
	return distance(fromId, toId, r.config.hashBits)
}
