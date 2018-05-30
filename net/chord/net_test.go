package chord

import (
	"testing"
	"time"
)

func TestTCPJoin(t *testing.T) {
	// Prepare to create 2 nodes
	c1, t1, err := prepRing(10025)
	if err != nil {
		t.Fatalf("unexpected err. %s", err)
	}
	c2, t2, err := prepRing(10026)
	if err != nil {
		t.Fatalf("unexpected err. %s", err)
	}

	// Create initial ring
	r1, err := Create(c1, t1)
	if err != nil {
		t.Fatalf("unexpected err. %s", err)
	}

	// Join ring
	r2, err := Join(c2, t2, c1.Hostname)
	if err != nil {
		t.Fatalf("failed to join local node! Got %s", err)
	}

	// Shutdown
	r1.Shutdown()
	r2.Shutdown()
	t1.Shutdown()
	t2.Shutdown()
}

func TestTCPLeave(t *testing.T) {
	// Prepare to create 2 nodes
	c1, t1, err := prepRing(10027)
	if err != nil {
		t.Fatalf("unexpected err. %s", err)
	}
	c2, t2, err := prepRing(10028)
	if err != nil {
		t.Fatalf("unexpected err. %s", err)
	}

	// Create initial ring
	r1, err := Create(c1, t1)
	if err != nil {
		t.Fatalf("unexpected err. %s", err)
	}

	// Join ring
	r2, err := Join(c2, t2, c1.Hostname)
	if err != nil {
		t.Fatalf("failed to join local node! Got %s", err)
	}

	// Wait for some stabilization
	<-time.After(100 * time.Millisecond)

	// Node 1 should leave
	r1.Leave()
	t1.Shutdown()

	// Wait for stabilization
	<-time.After(100 * time.Millisecond)

	// Verify r2 ring is still in tact
	for _, vn := range r2.Vnodes {
		if vn.successors[0].Host != r2.config.Hostname {
			t.Fatalf("bad successor! Got:%s:%s", vn.successors[0].Host,
				vn.successors[0])
		}
	}
}
