package chord

import (
	"crypto/sha1"
	"encoding/json"
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
		if ring.Vnodes[i] == nil {
			t.Fatalf("missing vnode!")
		}
		if ring.Vnodes[i].ring != ring {
			t.Fatalf("ring missing!")
		}
		if ring.Vnodes[i].Id == nil {
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
	if CompareId(ring.Vnodes[0].Id, ring.Vnodes[1].Id) != -1 {
		t.Fatalf("bad sort")
	}
	if CompareId(ring.Vnodes[1].Id, ring.Vnodes[2].Id) != -1 {
		t.Fatalf("bad sort")
	}
	if CompareId(ring.Vnodes[2].Id, ring.Vnodes[3].Id) != -1 {
		t.Fatalf("bad sort")
	}
	if CompareId(ring.Vnodes[3].Id, ring.Vnodes[4].Id) != -1 {
		t.Fatalf("bad sort")
	}
}

func TestRingToData(t *testing.T) {
	ring := makeRing()
	js, err := json.Marshal(ring.ToData())

	if err != nil {
		t.Fatal(err)
	} else {
		t.Logf("%s", js)
	}
}

func TestRingNearest(t *testing.T) {
	ring := makeRing()
	ring.Vnodes[0].Id = []byte{2}
	ring.Vnodes[1].Id = []byte{4}
	ring.Vnodes[2].Id = []byte{7}
	ring.Vnodes[3].Id = []byte{10}
	ring.Vnodes[4].Id = []byte{14}
	key := []byte{6}

	near := ring.nearestVnode(key)
	if near != ring.Vnodes[1] {
		t.Fatalf("got wrong node back!")
	}

	key = []byte{0}
	near = ring.nearestVnode(key)
	if near != ring.Vnodes[4] {
		t.Fatalf("got wrong node back!")
	}
}

func TestRingSchedule(t *testing.T) {
	ring := makeRing()
	ring.setLocalSuccessors()
	ring.schedule()
	for i := 0; i < len(ring.Vnodes); i++ {
		if ring.Vnodes[i].timer == nil {
			t.Fatalf("expected timer!")
		}
	}
	ring.stopVnodes()
}

func TestRingSetLocalSucc(t *testing.T) {
	ring := makeRing()
	ring.setLocalSuccessors()
	for i := 0; i < len(ring.Vnodes); i++ {
		for j := 0; j < 4; j++ {
			if ring.Vnodes[i].successors[j] == nil {
				t.Fatalf("expected successor!")
			}
		}
		if ring.Vnodes[i].successors[4] != nil {
			t.Fatalf("should not have 5th successor!")
		}
	}

	// Verify the successor manually for node 3
	vn := ring.Vnodes[2]
	if vn.successors[0] != &ring.Vnodes[3].Vnode {
		t.Fatalf("bad succ!")
	}
	if vn.successors[1] != &ring.Vnodes[4].Vnode {
		t.Fatalf("bad succ!")
	}
	if vn.successors[2] != &ring.Vnodes[0].Vnode {
		t.Fatalf("bad succ!")
	}
	if vn.successors[3] != &ring.Vnodes[1].Vnode {
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
