package chord

import (
	"bytes"
	"crypto/sha1"
	"sort"
	"testing"
	"time"
)

type MockDelegate struct {
	shutdown bool
}

func (m *MockDelegate) NewPredecessor(local, remoteNew, remotePrev *Vnode) {
}
func (m *MockDelegate) Leaving(local, pred, succ *Vnode) {
}
func (m *MockDelegate) PredecessorLeaving(local, remote *Vnode) {
}
func (m *MockDelegate) SuccessorLeaving(local, remote *Vnode) {
}
func (m *MockDelegate) Shutdown() {
	m.shutdown = true
}

func makeRing() *Ring {
	conf := &Config{
		NumVnodes:     5,
		NumSuccessors: 8,
		HashFunc:      sha1.New,
		hashBits:      160,
		StabilizeMin:  time.Second,
		StabilizeMax:  5 * time.Second,
	}

	ring := &Ring{}
	ring.init(conf, nil)
	return ring
}

func TestRingInit(t *testing.T) {
	// Create a ring
	ring := &Ring{}
	conf := DefaultConfig("test")
	ring.init(conf, nil)

	// Test features
	if ring.config != conf {
		t.Fatalf("wrong config")
	}
	if ring.transport == nil {
		t.Fatalf("missing transport")
	}

	// Check the vnodes
	for i := 0; i < conf.NumVnodes; i++ {
		if ring.vnodes[i] == nil {
			t.Fatalf("missing vnode!")
		}
		if ring.vnodes[i].ring != ring {
			t.Fatalf("ring missing!")
		}
		if ring.vnodes[i].Id == nil {
			t.Fatalf("ID not initialized!")
		}
	}
}

func TestRingLen(t *testing.T) {
	ring := makeRing()
	if ring.Len() != 5 {
		t.Fatalf("wrong len")
	}
}

func TestRingSort(t *testing.T) {
	ring := makeRing()
	sort.Sort(ring)
	if bytes.Compare(ring.vnodes[0].Id, ring.vnodes[1].Id) != -1 {
		t.Fatalf("bad sort")
	}
	if bytes.Compare(ring.vnodes[1].Id, ring.vnodes[2].Id) != -1 {
		t.Fatalf("bad sort")
	}
	if bytes.Compare(ring.vnodes[2].Id, ring.vnodes[3].Id) != -1 {
		t.Fatalf("bad sort")
	}
	if bytes.Compare(ring.vnodes[3].Id, ring.vnodes[4].Id) != -1 {
		t.Fatalf("bad sort")
	}
}

func TestRingNearest(t *testing.T) {
	ring := makeRing()
	ring.vnodes[0].Id = []byte{2}
	ring.vnodes[1].Id = []byte{4}
	ring.vnodes[2].Id = []byte{7}
	ring.vnodes[3].Id = []byte{10}
	ring.vnodes[4].Id = []byte{14}
	key := []byte{6}

	near := ring.nearestVnode(key)
	if near != ring.vnodes[1] {
		t.Fatalf("got wrong node back!")
	}

	key = []byte{0}
	near = ring.nearestVnode(key)
	if near != ring.vnodes[4] {
		t.Fatalf("got wrong node back!")
	}
}

func TestRingSchedule(t *testing.T) {
	ring := makeRing()
	ring.setLocalSuccessors()
	ring.schedule()
	for i := 0; i < len(ring.vnodes); i++ {
		if ring.vnodes[i].timer == nil {
			t.Fatalf("expected timer!")
		}
	}
	ring.stopVnodes()
}

func TestRingSetLocalSucc(t *testing.T) {
	ring := makeRing()
	ring.setLocalSuccessors()
	for i := 0; i < len(ring.vnodes); i++ {
		for j := 0; j < 4; j++ {
			if ring.vnodes[i].successors[j] == nil {
				t.Fatalf("expected successor!")
			}
		}
		if ring.vnodes[i].successors[4] != nil {
			t.Fatalf("should not have 5th successor!")
		}
	}

	// Verify the successor manually for node 3
	vn := ring.vnodes[2]
	if vn.successors[0] != &ring.vnodes[3].Vnode {
		t.Fatalf("bad succ!")
	}
	if vn.successors[1] != &ring.vnodes[4].Vnode {
		t.Fatalf("bad succ!")
	}
	if vn.successors[2] != &ring.vnodes[0].Vnode {
		t.Fatalf("bad succ!")
	}
	if vn.successors[3] != &ring.vnodes[1].Vnode {
		t.Fatalf("bad succ!")
	}
}

func TestRingDelegate(t *testing.T) {
	d := &MockDelegate{}
	ring := makeRing()
	ring.setLocalSuccessors()
	ring.config.Delegate = d
	ring.schedule()

	var b bool
	f := func() {
		println("run!")
		b = true
	}
	ch := ring.invokeDelegate(f)
	if ch == nil {
		t.Fatalf("expected chan")
	}
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}
	if !b {
		t.Fatalf("b should be true")
	}

	ring.stopDelegate()
	if !d.shutdown {
		t.Fatalf("delegate did not get shutdown")
	}
}
