package chord

import (
	"bytes"
	"crypto/sha256"
	"sort"
	"testing"
	"time"
)

func makeVnode() *localVnode {
	min := time.Duration(10 * time.Second)
	max := time.Duration(30 * time.Second)
	conf := &Config{
		NumSuccessors: 8,
		StabilizeMin:  min,
		StabilizeMax:  max,
		HashFunc:      sha256.New,
		Hostname:      "127.0.0.1:30000",
		JoinBlkHeight: 0,
	}
	trans := InitLocalTransport(nil)
	ring := &Ring{config: conf, transport: trans}
	return &localVnode{ring: ring}
}

func TestVnodeInit(t *testing.T) {
	vn := makeVnode()
	vn.init(0)
	if vn.Id == nil {
		t.Fatalf("unexpected nil")
	}
	if vn.successors == nil {
		t.Fatalf("unexpected nil")
	}
	if vn.finger == nil {
		t.Fatalf("unexpected nil")
	}
	if vn.timer != nil {
		t.Fatalf("unexpected timer")
	}
}

func TestVnodeSchedule(t *testing.T) {
	vn := makeVnode()
	vn.schedule()
	if vn.timer == nil {
		t.Fatalf("unexpected nil")
	}
}

func TestGenId(t *testing.T) {
	vn := makeVnode()
	var ids [][]byte
	for i := 0; i < 16; i++ {
		vn.genId(vn.ring.config.Hostname, uint32(i))
		ids = append(ids, vn.Id)
	}

	for idx, val := range ids {
		for i := 0; i < len(ids); i++ {
			if idx != i && bytes.Compare(ids[i], val) == 0 {
				t.Fatalf("unexpected id collision!")
			}
		}
	}
}

func TestVnodeStabilizeShutdown(t *testing.T) {
	vn := makeVnode()
	vn.schedule()
	vn.ring.shutdown = make(chan bool, 1)
	vn.stabilize()

	if vn.timer != nil {
		t.Fatalf("unexpected timer")
	}
	if !vn.stabilized.IsZero() {
		t.Fatalf("unexpected time")
	}
	select {
	case <-vn.ring.shutdown:
		return
	default:
		t.Fatalf("expected message")
	}
}

func TestVnodeStabilizeResched(t *testing.T) {
	vn := makeVnode()
	vn.init(1)
	vn.successors[0] = &vn.Vnode
	vn.schedule()
	vn.stabilize()

	if vn.timer == nil {
		t.Fatalf("expected timer")
	}
	if vn.stabilized.IsZero() {
		t.Fatalf("expected time")
	}
	vn.timer.Stop()
}

func TestVnodeKnownSucc(t *testing.T) {
	vn := makeVnode()
	vn.init(0)
	if vn.knownSuccessors() != 0 {
		t.Fatalf("wrong num known!")
	}
	vn.successors[0] = &Vnode{Id: []byte{1}}
	if vn.knownSuccessors() != 1 {
		t.Fatalf("wrong num known!")
	}
}

// Checks panic if no successors
func TestVnodeCheckNewSuccAlivePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic!")
		}
	}()
	vn1 := makeVnode()
	vn1.init(1)
	vn1.checkNewSuccessor()
}

// Checks pinging a live successor with no changes
func TestVnodeCheckNewSuccAlive(t *testing.T) {
	vn1 := makeVnode()
	vn1.init(1)

	vn2 := makeVnode()
	vn2.ring = vn1.ring
	vn2.init(2)
	vn2.predecessor = &vn1.Vnode
	vn1.successors[0] = &vn2.Vnode

	if pred, _ := vn2.GetPredecessor(); pred != &vn1.Vnode {
		t.Fatalf("expected vn1 as predecessor")
	}

	if err := vn1.checkNewSuccessor(); err != nil {
		t.Fatalf("unexpected err %s", err)
	}

	if vn1.successors[0] != &vn2.Vnode {
		t.Fatalf("unexpected successor!")
	}
}

// Checks pinging a dead successor with no alternates
func TestVnodeCheckNewSuccDead(t *testing.T) {
	vn1 := makeVnode()
	vn1.init(1)
	vn1.successors[0] = &Vnode{Id: []byte{0}}

	if err := vn1.checkNewSuccessor(); err == nil {
		t.Fatalf("err: %s", err)
	}

	if vn1.successors[0].String() != "00" {
		t.Fatalf("unexpected successor!")
	}
}

// Checks pinging a dead successor with alternate
func TestVnodeCheckNewSuccDeadAlternate(t *testing.T) {
	r := makeRing()
	sort.Sort(r)

	vn1 := r.Vnodes[0]
	vn2 := r.Vnodes[1]
	vn3 := r.Vnodes[2]

	vn1.successors[0] = &vn2.Vnode
	vn1.successors[1] = &vn3.Vnode
	vn2.predecessor = &vn1.Vnode
	vn3.predecessor = &vn2.Vnode

	// Remove vn2
	(r.transport.(*LocalTransport)).Deregister(&vn2.Vnode)

	// Should not get an error
	if err := vn1.checkNewSuccessor(); err != nil {
		t.Fatalf("unexpected err %s", err)
	}

	// Should become vn3
	if vn1.successors[0] != &vn3.Vnode {
		t.Fatalf("unexpected successor!")
	}
}

// Checks pinging a dead successor with all dead alternates
func TestVnodeCheckNewSuccAllDeadAlternates(t *testing.T) {
	r := makeRing()
	sort.Sort(r)

	vn1 := r.Vnodes[0]
	vn2 := r.Vnodes[1]
	vn3 := r.Vnodes[2]

	vn1.successors[0] = &vn2.Vnode
	vn1.successors[1] = &vn3.Vnode
	vn2.predecessor = &vn1.Vnode
	vn3.predecessor = &vn2.Vnode

	// Remove vn2
	(r.transport.(*LocalTransport)).Deregister(&vn2.Vnode)
	(r.transport.(*LocalTransport)).Deregister(&vn3.Vnode)

	// Should get an error
	if err := vn1.checkNewSuccessor(); err.Error() != "All known successors dead!" {
		t.Fatalf("unexpected err %s", err)
	}

	// Should just be vn3
	if vn1.successors[0] != &vn3.Vnode {
		t.Fatalf("unexpected successor!")
	}
}

// Checks pinging a successor, and getting a new successor
func TestVnodeCheckNewSuccNewSucc(t *testing.T) {
	r := makeRing()
	sort.Sort(r)

	vn1 := r.Vnodes[0]
	vn2 := r.Vnodes[1]
	vn3 := r.Vnodes[2]

	vn1.successors[0] = &vn3.Vnode
	vn2.predecessor = &vn1.Vnode
	vn3.predecessor = &vn2.Vnode

	// vn3 pred is vn2
	if pred, _ := vn3.GetPredecessor(); pred != &vn2.Vnode {
		t.Fatalf("expected vn2 as predecessor")
	}

	// Should not get an error
	if err := vn1.checkNewSuccessor(); err != nil {
		t.Fatalf("unexpected err %s", err)
	}

	// Should become vn2
	if vn1.successors[0] != &vn2.Vnode {
		t.Fatalf("unexpected successor! %s", vn1.successors[0])
	}

	// 2nd successor should become vn3
	if vn1.successors[1] != &vn3.Vnode {
		t.Fatalf("unexpected 2nd successor!")
	}
}

// Checks pinging a successor, and getting a new successor
// which is not alive
func TestVnodeCheckNewSuccNewSuccDead(t *testing.T) {
	r := makeRing()
	sort.Sort(r)

	vn1 := r.Vnodes[0]
	vn2 := r.Vnodes[1]
	vn3 := r.Vnodes[2]

	vn1.successors[0] = &vn3.Vnode
	vn2.predecessor = &vn1.Vnode
	vn3.predecessor = &vn2.Vnode

	// Remove vn2
	(r.transport.(*LocalTransport)).Deregister(&vn2.Vnode)

	// Should not get an error
	if err := vn1.checkNewSuccessor(); err != nil {
		t.Fatalf("unexpected err %s", err)
	}

	// Should stay vn3
	if vn1.successors[0] != &vn3.Vnode {
		t.Fatalf("unexpected successor!")
	}
}

// Test notifying a successor successfully
func TestVnodeNotifySucc(t *testing.T) {
	r := makeRing()
	sort.Sort(r)

	s1 := &Vnode{Id: []byte{1}}
	s2 := &Vnode{Id: []byte{2}}
	s3 := &Vnode{Id: []byte{3}}

	vn1 := r.Vnodes[0]
	vn2 := r.Vnodes[1]
	vn1.successors[0] = &vn2.Vnode
	vn2.predecessor = &vn1.Vnode
	vn2.successors[0] = s1
	vn2.successors[1] = s2
	vn2.successors[2] = s3

	// Should get no error
	if err := vn1.notifySuccessor(); err != nil {
		t.Fatalf("unexpected err %s", err)
	}

	// Successor list should be updated
	if vn1.successors[1] != s1 {
		t.Fatalf("bad succ 1")
	}
	if vn1.successors[2] != s2 {
		t.Fatalf("bad succ 2")
	}
	if vn1.successors[3] != s3 {
		t.Fatalf("bad succ 3")
	}

	// Predecessor should not updated
	if vn2.predecessor != &vn1.Vnode {
		t.Fatalf("bad predecessor")
	}
}

// Test notifying a dead successor
func TestVnodeNotifySuccDead(t *testing.T) {
	r := makeRing()
	sort.Sort(r)

	vn1 := r.Vnodes[0]
	vn2 := r.Vnodes[1]
	vn1.successors[0] = &vn2.Vnode
	vn2.predecessor = &vn1.Vnode

	// Remove vn2
	(r.transport.(*LocalTransport)).Deregister(&vn2.Vnode)

	// Should get error
	if err := vn1.notifySuccessor(); err == nil {
		t.Fatalf("expected err!")
	}
}

func TestVnodeNotifySamePred(t *testing.T) {
	r := makeRing()
	sort.Sort(r)

	s1 := &Vnode{Id: []byte{1}}
	s2 := &Vnode{Id: []byte{2}}
	s3 := &Vnode{Id: []byte{3}}

	vn1 := r.Vnodes[0]
	vn2 := r.Vnodes[1]
	vn1.successors[0] = &vn2.Vnode
	vn2.predecessor = &vn1.Vnode
	vn2.successors[0] = s1
	vn2.successors[1] = s2
	vn2.successors[2] = s3

	succs, err := vn2.Notify(&vn1.Vnode)
	if err != nil {
		t.Fatalf("unexpected error! %s", err)
	}
	if succs[0] != s1 {
		t.Fatalf("unexpected succ 0")
	}
	if succs[1] != s2 {
		t.Fatalf("unexpected succ 1")
	}
	if succs[2] != s3 {
		t.Fatalf("unexpected succ 2")
	}
	if vn2.predecessor != &vn1.Vnode {
		t.Fatalf("unexpected pred")
	}
}

func TestVnodeNotifyNoPred(t *testing.T) {
	r := makeRing()
	sort.Sort(r)

	s1 := &Vnode{Id: []byte{1}}
	s2 := &Vnode{Id: []byte{2}}
	s3 := &Vnode{Id: []byte{3}}

	vn1 := r.Vnodes[0]
	vn2 := r.Vnodes[1]
	vn2.successors[0] = s1
	vn2.successors[1] = s2
	vn2.successors[2] = s3

	succs, err := vn2.Notify(&vn1.Vnode)
	if err != nil {
		t.Fatalf("unexpected error! %s", err)
	}
	if succs[0] != s1 {
		t.Fatalf("unexpected succ 0")
	}
	if succs[1] != s2 {
		t.Fatalf("unexpected succ 1")
	}
	if succs[2] != s3 {
		t.Fatalf("unexpected succ 2")
	}
	if vn2.predecessor != &vn1.Vnode {
		t.Fatalf("unexpected pred")
	}
}

func TestVnodeNotifyNewPred(t *testing.T) {
	r := makeRing()
	sort.Sort(r)

	vn1 := r.Vnodes[0]
	vn2 := r.Vnodes[1]
	vn3 := r.Vnodes[2]
	vn3.predecessor = &vn1.Vnode

	_, err := vn3.Notify(&vn2.Vnode)
	if err != nil {
		t.Fatalf("unexpected error! %s", err)
	}
	if vn3.predecessor != &vn2.Vnode {
		t.Fatalf("unexpected pred")
	}
}

func TestVnodeFixFinger(t *testing.T) {
	r := makeRing()
	sort.Sort(r)
	num := len(r.Vnodes)
	for i := 0; i < num; i++ {
		r.Vnodes[i].init(i)
		r.Vnodes[i].successors[0] = &r.Vnodes[(i+1)%num].Vnode
	}

	// Fix finger should not error
	vn := r.Vnodes[0]
	if err := vn.fixFingerTable(); err != nil {
		t.Fatalf("unexpected err, %s", err)
	}

	// Check we've progressed
	if vn.last_finger != 158 {
		t.Fatalf("unexpected last finger! %d", vn.last_finger)
	}

	// Ensure that we've setup our successor as the initial entries
	for i := 0; i < vn.last_finger; i++ {
		if vn.finger[i] != vn.successors[0] {
			t.Fatalf("unexpected finger entry!")
		}
	}

	// Fix next index
	if err := vn.fixFingerTable(); err != nil {
		t.Fatalf("unexpected err, %s", err)
	}
	if vn.last_finger != 0 {
		t.Fatalf("unexpected last finger! %d", vn.last_finger)
	}
}

func TestVnodeCheckPredNoPred(t *testing.T) {
	v := makeVnode()
	v.init(0)
	if err := v.checkPredecessor(); err != nil {
		t.Fatalf("unpexected err! %s", err)
	}
}

func TestVnodeCheckLivePred(t *testing.T) {
	r := makeRing()
	sort.Sort(r)

	vn1 := r.Vnodes[0]
	vn2 := r.Vnodes[1]
	vn2.predecessor = &vn1.Vnode

	if err := vn2.checkPredecessor(); err != nil {
		t.Fatalf("unexpected error! %s", err)
	}
	if vn2.predecessor != &vn1.Vnode {
		t.Fatalf("unexpected pred")
	}
}

func TestVnodeCheckDeadPred(t *testing.T) {
	r := makeRing()
	sort.Sort(r)

	vn1 := r.Vnodes[0]
	vn2 := r.Vnodes[1]
	vn2.predecessor = &vn1.Vnode

	// Deregister vn1
	(r.transport.(*LocalTransport)).Deregister(&vn1.Vnode)

	if err := vn2.checkPredecessor(); err != nil {
		t.Fatalf("unexpected error! %s", err)
	}
	if vn2.predecessor != nil {
		t.Fatalf("unexpected pred")
	}
}

func TestVnodeFindSuccessors(t *testing.T) {
	r := makeRing()
	sort.Sort(r)
	num := len(r.Vnodes)
	for i := 0; i < num; i++ {
		r.Vnodes[i].successors[0] = &r.Vnodes[(i+1)%num].Vnode
	}

	// Get a random key
	h := r.config.HashFunc()
	h.Write([]byte("test"))
	key := h.Sum(nil)

	// Local only, should be nearest in the ring
	nearest := r.nearestVnode(key)
	exp := nearest.successors[0]

	// Do a lookup on the key
	for i := 0; i < len(r.Vnodes); i++ {
		vn := r.Vnodes[i]
		succ, err := vn.FindSuccessors(1, key)
		if err != nil {
			t.Fatalf("unexpected err! %s", err)
		}

		// Local only, should be nearest in the ring
		if exp != succ[0] {
			t.Fatalf("unexpected succ! K:%x Exp: %s Got:%s",
				key, exp, succ[0])
		}
	}
}

// Ensure each node has multiple successors
func TestVnodeFindSuccessorsMultSucc(t *testing.T) {
	r := makeRing()
	sort.Sort(r)
	num := len(r.Vnodes)
	for i := 0; i < num; i++ {
		r.Vnodes[i].successors[0] = &r.Vnodes[(i+1)%num].Vnode
		r.Vnodes[i].successors[1] = &r.Vnodes[(i+2)%num].Vnode
		r.Vnodes[i].successors[2] = &r.Vnodes[(i+3)%num].Vnode
	}

	// Get a random key
	h := r.config.HashFunc()
	h.Write([]byte("test"))
	key := h.Sum(nil)

	// Local only, should be nearest in the ring
	nearest := r.nearestVnode(key)
	exp := nearest.successors[0]

	// Do a lookup on the key
	for i := 0; i < len(r.Vnodes); i++ {
		vn := r.Vnodes[i]
		succ, err := vn.FindSuccessors(1, key)
		if err != nil {
			t.Fatalf("unexpected err! %s", err)
		}

		// Local only, should be nearest in the ring
		if exp != succ[0] {
			t.Fatalf("unexpected succ! K:%x Exp: %s Got:%s",
				key, exp, succ[0])
		}
	}
}

// Kill off a part of the ring and see what happens
func TestVnodeFindSuccessorsSomeDead(t *testing.T) {
	r := makeRing()
	sort.Sort(r)
	num := len(r.Vnodes)
	for i := 0; i < num; i++ {
		r.Vnodes[i].successors[0] = &r.Vnodes[(i+1)%num].Vnode
		r.Vnodes[i].successors[1] = &r.Vnodes[(i+2)%num].Vnode
	}

	// Kill 2 of the nodes
	(r.transport.(*LocalTransport)).Deregister(&r.Vnodes[0].Vnode)
	(r.transport.(*LocalTransport)).Deregister(&r.Vnodes[3].Vnode)

	// Get a random key
	h := r.config.HashFunc()
	h.Write([]byte("test"))
	key := h.Sum(nil)

	// Local only, should be nearest in the ring
	nearest := r.nearestVnode(key)
	exp := nearest.successors[0]

	// Do a lookup on the key
	for i := 0; i < len(r.Vnodes); i++ {
		vn := r.Vnodes[i]
		succ, err := vn.FindSuccessors(1, key)
		if err != nil {
			t.Fatalf("(%d) unexpected err! %s", i, err)
		}

		// Local only, should be nearest in the ring
		if exp != succ[0] {
			t.Fatalf("(%d) unexpected succ! K:%x Exp: %s Got:%s",
				i, key, exp, succ[0])
		}
	}
}

func TestVnodeClearPred(t *testing.T) {
	v := makeVnode()
	v.init(0)
	p := &Vnode{Id: []byte{12}}
	v.predecessor = p
	v.ClearPredecessor(p)
	if v.predecessor != nil {
		t.Fatalf("expect no predecessor!")
	}

	np := &Vnode{Id: []byte{14}}
	v.predecessor = p
	v.ClearPredecessor(np)
	if v.predecessor != p {
		t.Fatalf("expect p predecessor!")
	}
}

func TestVnodeSkipSucc(t *testing.T) {
	v := makeVnode()
	v.init(0)

	s1 := &Vnode{Id: []byte{10}}
	s2 := &Vnode{Id: []byte{11}}
	s3 := &Vnode{Id: []byte{12}}

	v.successors[0] = s1
	v.successors[1] = s2
	v.successors[2] = s3

	// s2 should do nothing
	if err := v.SkipSuccessor(s2); err != nil {
		t.Fatalf("unexpected err")
	}
	if v.successors[0] != s1 {
		t.Fatalf("unexpected suc")
	}

	// s1 should skip
	if err := v.SkipSuccessor(s1); err != nil {
		t.Fatalf("unexpected err")
	}
	if v.successors[0] != s2 {
		t.Fatalf("unexpected suc")
	}
	if v.knownSuccessors() != 2 {
		t.Fatalf("bad num of suc")
	}
}

func TestVnodeLeave(t *testing.T) {
	r := makeRing()
	sort.Sort(r)
	num := len(r.Vnodes)
	for i := int(0); i < num; i++ {
		r.Vnodes[i].predecessor = &r.Vnodes[(i+num-1)%num].Vnode
		r.Vnodes[i].successors[0] = &r.Vnodes[(i+1)%num].Vnode
		r.Vnodes[i].successors[1] = &r.Vnodes[(i+2)%num].Vnode
	}

	// Make node 0 leave
	if err := r.Vnodes[0].leave(); err != nil {
		t.Fatalf("unexpected err")
	}

	if r.Vnodes[4].successors[0] != &r.Vnodes[1].Vnode {
		t.Fatalf("unexpected suc!")
	}
	if r.Vnodes[1].predecessor != nil {
		t.Fatalf("unexpected pred!")
	}
}
