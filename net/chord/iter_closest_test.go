package chord

import (
	"math/big"
	"testing"
)

func TestNextClosest(t *testing.T) {
	// Make the vnodes on the ring (mod 64)
	v1 := &Vnode{Id: []byte{1}}
	v2 := &Vnode{Id: []byte{10}}
	//v3 := &Vnode{Id: []byte{20}}
	v4 := &Vnode{Id: []byte{32}}
	//v5 := &Vnode{Id: []byte{40}}
	v6 := &Vnode{Id: []byte{59}}
	v7 := &Vnode{Id: []byte{62}}

	// Make a vnode
	vn := &localVnode{}
	vn.Id = []byte{54}
	vn.successors = []*Vnode{v6, v7, nil}
	vn.finger = []*Vnode{v6, v6, v7, v1, v2, v4, nil}
	vn.ring = &Ring{}
	vn.ring.config = &Config{hashBits: 6}

	// Make an iterator
	k := []byte{32}
	cp := &closestPreceedingVnodeIterator{}
	cp.init(vn, k)

	// Iterate until we are done
	s1 := cp.Next()
	if s1 != v2 {
		t.Fatalf("Expect v2. %v", s1)
	}

	s2 := cp.Next()
	if s2 != v1 {
		t.Fatalf("Expect v1. %v", s2)
	}

	s3 := cp.Next()
	if s3 != v7 {
		t.Fatalf("Expect v7. %v", s3)
	}

	s4 := cp.Next()
	if s4 != v6 {
		t.Fatalf("Expect v6. %v", s4)
	}

	s5 := cp.Next()
	if s5 != nil {
		t.Fatalf("Expect nil. %v", s5)
	}
}

func TestNextClosestNoSucc(t *testing.T) {
	// Make the vnodes on the ring (mod 64)
	v1 := &Vnode{Id: []byte{1}}
	v2 := &Vnode{Id: []byte{10}}
	//v3 := &Vnode{Id: []byte{20}}
	v4 := &Vnode{Id: []byte{32}}
	//v5 := &Vnode{Id: []byte{40}}
	v6 := &Vnode{Id: []byte{59}}
	v7 := &Vnode{Id: []byte{62}}

	// Make a vnode
	vn := &localVnode{}
	vn.Id = []byte{54}
	vn.successors = []*Vnode{nil}
	vn.finger = []*Vnode{v6, v6, v7, v1, v2, v4, nil}
	vn.ring = &Ring{}
	vn.ring.config = &Config{hashBits: 6}

	// Make an iterator
	k := []byte{32}
	cp := &closestPreceedingVnodeIterator{}
	cp.init(vn, k)

	// Iterate until we are done
	s1 := cp.Next()
	if s1 != v2 {
		t.Fatalf("Expect v2. %v", s1)
	}

	s2 := cp.Next()
	if s2 != v1 {
		t.Fatalf("Expect v1. %v", s2)
	}

	s3 := cp.Next()
	if s3 != v7 {
		t.Fatalf("Expect v7. %v", s3)
	}

	s4 := cp.Next()
	if s4 != v6 {
		t.Fatalf("Expect v6. %v", s4)
	}

	s5 := cp.Next()
	if s5 != nil {
		t.Fatalf("Expect nil. %v", s5)
	}
}

func TestNextClosestNoFinger(t *testing.T) {
	// Make the vnodes on the ring (mod 64)
	//v1 := &Vnode{Id: []byte{1}}
	//v2 := &Vnode{Id: []byte{10}}
	//v3 := &Vnode{Id: []byte{20}}
	//v4 := &Vnode{Id: []byte{32}}
	//v5 := &Vnode{Id: []byte{40}}
	v6 := &Vnode{Id: []byte{59}}
	v7 := &Vnode{Id: []byte{62}}

	// Make a vnode
	vn := &localVnode{}
	vn.Id = []byte{54}
	vn.successors = []*Vnode{v6, v7, v7, nil}
	vn.finger = []*Vnode{nil, nil, nil}
	vn.ring = &Ring{}
	vn.ring.config = &Config{hashBits: 6}

	// Make an iterator
	k := []byte{32}
	cp := &closestPreceedingVnodeIterator{}
	cp.init(vn, k)

	// Iterate until we are done
	s3 := cp.Next()
	if s3 != v7 {
		t.Fatalf("Expect v7. %v", s3)
	}

	s4 := cp.Next()
	if s4 != v6 {
		t.Fatalf("Expect v6. %v", s4)
	}

	s5 := cp.Next()
	if s5 != nil {
		t.Fatalf("Expect nil. %v", s5)
	}
}

func TestClosest(t *testing.T) {
	a := &Vnode{Id: []byte{128}}
	b := &Vnode{Id: []byte{32}}
	k := []byte{45}
	c := closest_preceeding_vnode(a, b, k, 8)
	if c != b {
		t.Fatalf("expect b to be closer!")
	}
	c = closest_preceeding_vnode(b, a, k, 8)
	if c != b {
		t.Fatalf("expect b to be closer!")
	}
}

func TestDistance(t *testing.T) {
	a := []byte{63}
	b := []byte{3}
	d := distance(a, b, 6) // Ring size of 64
	if d.Cmp(big.NewInt(4)) != 0 {
		t.Fatalf("expect distance 4! %v", d)
	}

	a = []byte{0}
	b = []byte{65}
	d = distance(a, b, 7) // Ring size of 128
	if d.Cmp(big.NewInt(65)) != 0 {
		t.Fatalf("expect distance 65! %v", d)
	}

	a = []byte{1}
	b = []byte{255}
	d = distance(a, b, 8) // Ring size of 256
	if d.Cmp(big.NewInt(254)) != 0 {
		t.Fatalf("expect distance 254! %v", d)
	}
}
