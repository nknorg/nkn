package chord

import (
	"errors"
	"testing"
	"time"
)

func TestRandStabilize(t *testing.T) {
	min := time.Duration(10 * time.Second)
	max := time.Duration(30 * time.Second)
	conf := &Config{
		StabilizeMin: min,
		StabilizeMax: max}

	var times []time.Duration
	for i := 0; i < 1000; i++ {
		after := randStabilize(conf)
		times = append(times, after)
		if after < min {
			t.Fatalf("after below min")
		}
		if after > max {
			t.Fatalf("after above max")
		}
	}

	collisions := 0
	for idx, val := range times {
		for i := 0; i < len(times); i++ {
			if idx != i && times[i] == val {
				collisions += 1
			}
		}
	}

	if collisions > 3 {
		t.Fatalf("too many collisions! %d", collisions)
	}
}

func TestBetween(t *testing.T) {
	t1 := []byte{0, 0, 0, 0}
	t2 := []byte{1, 0, 0, 0}
	k := []byte{0, 0, 5, 0}
	if !between(t1, t2, k) {
		t.Fatalf("expected k between!")
	}
	if between(t1, t2, t1) {
		t.Fatalf("dont expect t1 between!")
	}
	if between(t1, t2, t2) {
		t.Fatalf("dont expect t1 between!")
	}

	k = []byte{2, 0, 0, 0}
	if between(t1, t2, k) {
		t.Fatalf("dont expect k between!")
	}
}

func TestBetweenWrap(t *testing.T) {
	t1 := []byte{0xff, 0, 0, 0}
	t2 := []byte{1, 0, 0, 0}
	k := []byte{0, 0, 5, 0}
	if !between(t1, t2, k) {
		t.Fatalf("expected k between!")
	}

	k = []byte{0xff, 0xff, 0, 0}
	if !between(t1, t2, k) {
		t.Fatalf("expect k between!")
	}
}

func TestBetweenRightIncl(t *testing.T) {
	t1 := []byte{0, 0, 0, 0}
	t2 := []byte{1, 0, 0, 0}
	k := []byte{1, 0, 0, 0}
	if !betweenRightIncl(t1, t2, k) {
		t.Fatalf("expected k between!")
	}
}

func TestBetweenRightInclWrap(t *testing.T) {
	t1 := []byte{0xff, 0, 0, 0}
	t2 := []byte{1, 0, 0, 0}
	k := []byte{1, 0, 0, 0}
	if !betweenRightIncl(t1, t2, k) {
		t.Fatalf("expected k between!")
	}
}

func TestPowerOffset(t *testing.T) {
	id := []byte{0, 0, 0, 0}
	exp := 30
	mod := 32
	val := powerOffset(id, exp, mod)
	if val[0] != 64 {
		t.Fatalf("unexpected val! %v", val)
	}

	// 0-7, 8-15, 16-23, 24-31
	id = []byte{0, 0xff, 0xff, 0xff}
	exp = 23
	val = powerOffset(id, exp, mod)
	if val[0] != 1 || val[1] != 0x7f || val[2] != 0xff || val[3] != 0xff {
		t.Fatalf("unexpected val! %v", val)
	}
}

func TestMax(t *testing.T) {
	if max(-10, 10) != 10 {
		t.Fatalf("bad max")
	}
	if max(10, -10) != 10 {
		t.Fatalf("bad max")
	}
}

func TestMin(t *testing.T) {
	if min(-10, 10) != -10 {
		t.Fatalf("bad min")
	}
	if min(10, -10) != -10 {
		t.Fatalf("bad min")
	}
}

func TestNearestVnodesKey(t *testing.T) {
	vnodes := make([]*Vnode, 5)
	vnodes[0] = &Vnode{Id: []byte{2}}
	vnodes[1] = &Vnode{Id: []byte{4}}
	vnodes[2] = &Vnode{Id: []byte{7}}
	vnodes[3] = &Vnode{Id: []byte{10}}
	vnodes[4] = &Vnode{Id: []byte{14}}
	key := []byte{6}

	near := nearestVnodeToKey(vnodes, key)
	if near != vnodes[1] {
		t.Fatalf("got wrong node back!")
	}

	key = []byte{0}
	near = nearestVnodeToKey(vnodes, key)
	if near != vnodes[4] {
		t.Fatalf("got wrong node back!")
	}
}

func TestMergeErrors(t *testing.T) {
	e1 := errors.New("test1")
	e2 := errors.New("test2")

	if mergeErrors(e1, nil) != e1 {
		t.Fatalf("bad merge")
	}
	if mergeErrors(nil, e1) != e1 {
		t.Fatalf("bad merge")
	}
	if mergeErrors(nil, nil) != nil {
		t.Fatalf("bad merge")
	}
	if mergeErrors(e1, e2).Error() != "test1\ntest2" {
		t.Fatalf("bad merge")
	}
}
