package chord

import (
	"runtime"
	"testing"
	"time"

	"github.com/nknorg/nkn/util/log"
)

func init() {
	log.Init(log.Stdout)
}

type MultiLocalTrans struct {
	remote Transport
	hosts  map[string]*LocalTransport
}

func InitMLTransport() *MultiLocalTrans {
	hosts := make(map[string]*LocalTransport)
	remote := &BlackholeTransport{}
	ml := &MultiLocalTrans{hosts: hosts}
	ml.remote = remote
	return ml
}

func (ml *MultiLocalTrans) ListVnodes(host string) ([]*Vnode, error) {
	if local, ok := ml.hosts[host]; ok {
		return local.ListVnodes(host)
	}
	return ml.remote.ListVnodes(host)
}

// Ping a Vnode, check for liveness
func (ml *MultiLocalTrans) Ping(v *Vnode) (bool, error) {
	if local, ok := ml.hosts[v.Host]; ok {
		return local.Ping(v)
	}
	return ml.remote.Ping(v)
}

// Request a nodes predecessor
func (ml *MultiLocalTrans) GetPredecessor(v *Vnode) (*Vnode, error) {
	if local, ok := ml.hosts[v.Host]; ok {
		return local.GetPredecessor(v)
	}
	return ml.remote.GetPredecessor(v)
}

// Notify our successor of ourselves
func (ml *MultiLocalTrans) Notify(target, self *Vnode) ([]*Vnode, error) {
	if local, ok := ml.hosts[target.Host]; ok {
		return local.Notify(target, self)
	}
	return ml.remote.Notify(target, self)
}

// Find a successor
func (ml *MultiLocalTrans) FindSuccessors(v *Vnode, n int, k []byte) ([]*Vnode, error) {
	if local, ok := ml.hosts[v.Host]; ok {
		return local.FindSuccessors(v, n, k)
	}
	return ml.remote.FindSuccessors(v, n, k)
}

// Clears a predecessor if it matches a given vnode. Used to leave.
func (ml *MultiLocalTrans) ClearPredecessor(target, self *Vnode) error {
	if local, ok := ml.hosts[target.Host]; ok {
		return local.ClearPredecessor(target, self)
	}
	return ml.remote.ClearPredecessor(target, self)
}

// Instructs a node to skip a given successor. Used to leave.
func (ml *MultiLocalTrans) SkipSuccessor(target, self *Vnode) error {
	if local, ok := ml.hosts[target.Host]; ok {
		return local.SkipSuccessor(target, self)
	}
	return ml.remote.SkipSuccessor(target, self)
}

func (ml *MultiLocalTrans) Register(v *Vnode, o VnodeRPC) {
	local, ok := ml.hosts[v.Host]
	if !ok {
		local = InitLocalTransport(nil).(*LocalTransport)
		ml.hosts[v.Host] = local
	}
	local.Register(v, o)
}

func (ml *MultiLocalTrans) Deregister(host string) {
	delete(ml.hosts, host)
}

func TestDefaultConfig(t *testing.T) {
	conf := DefaultConfig("test", false)
	if conf.Hostname != "test" {
		t.Fatalf("bad hostname")
	}
	if conf.NumVnodes != 1 {
		t.Fatalf("bad num vnodes")
	}
	if conf.NumSuccessors != 8 {
		t.Fatalf("bad num succ")
	}
	if conf.HashFunc == nil {
		t.Fatalf("bad hash")
	}
	if conf.hashBits != 256 {
		t.Fatalf("bad hash bits")
	}
	if conf.StabilizeMin != 150*time.Millisecond {
		t.Fatalf("bad min stable")
	}
	if conf.StabilizeMax != 450*time.Millisecond {
		t.Fatalf("bad max stable")
	}
	if conf.Delegate != nil {
		t.Fatalf("bad delegate")
	}
}

func fastConf() *Config {
	conf := DefaultConfig("test", true)
	conf.StabilizeMin = time.Duration(15 * time.Millisecond)
	conf.StabilizeMax = time.Duration(45 * time.Millisecond)
	conf.NumVnodes = 8
	return conf
}

func TestCreateShutdown(t *testing.T) {
	// Start the timer thread
	time.After(15)
	conf := fastConf()
	numGo := runtime.NumGoroutine()
	r, err := Create(conf, nil)
	if err != nil {
		t.Fatalf("unexpected err. %s", err)
	}
	r.Shutdown()
	after := runtime.NumGoroutine()
	if after != numGo {
		t.Fatalf("unexpected routines! A:%d B:%d", after, numGo)
	}
}

func TestJoin(t *testing.T) {
	// Create a multi transport
	ml := InitMLTransport()

	// Create the initial ring
	conf := fastConf()
	r, err := Create(conf, ml)
	if err != nil {
		t.Fatalf("unexpected err. %s", err)
	}

	// Create a second ring
	conf2 := fastConf()
	conf2.Hostname = "test2"
	r2, err := Join(conf2, ml, "test")
	if err != nil {
		t.Fatalf("failed to join local node! Got %s", err)
	}

	// Shutdown
	r.Shutdown()
	r2.Shutdown()
}

func TestJoinDeadHost(t *testing.T) {
	// Create a multi transport
	ml := InitMLTransport()

	// Create the initial ring
	conf := fastConf()
	_, err := Join(conf, ml, "noop")
	if err == nil {
		t.Fatalf("expected err!")
	}
}

func TestLeave(t *testing.T) {
	// Create a multi transport
	ml := InitMLTransport()

	// Create the initial ring
	conf := fastConf()
	r, err := Create(conf, ml)
	if err != nil {
		t.Fatalf("unexpected err. %s", err)
	}

	// Create a second ring
	conf2 := fastConf()
	conf2.Hostname = "test2"
	r2, err := Join(conf2, ml, "test")
	if err != nil {
		t.Fatalf("failed to join local node! Got %s", err)
	}

	// Wait for some stabilization
	<-time.After(100 * time.Millisecond)

	// Node 1 should leave
	r.Leave()
	ml.Deregister("test")

	// Wait for stabilization
	<-time.After(100 * time.Millisecond)

	// Verify r2 ring is still in tact
	num := len(r2.Vnodes)
	for idx, vn := range r2.Vnodes {
		if vn.successors[0] != &r2.Vnodes[(idx+1)%num].Vnode {
			t.Fatalf("bad successor! Got:%s:%s", vn.successors[0].Host,
				vn.successors[0])
		}
	}
}

func TestLookupBadN(t *testing.T) {
	// Create a multi transport
	ml := InitMLTransport()

	// Create the initial ring
	conf := fastConf()
	r, err := Create(conf, ml)
	if err != nil {
		t.Fatalf("unexpected err. %s", err)
	}

	_, err = r.Lookup(10, []byte("test"))
	if err == nil {
		t.Fatalf("expected err!")
	}
}

func TestLookup(t *testing.T) {
	// Create a multi transport
	ml := InitMLTransport()

	// Create the initial ring
	conf := fastConf()
	r, err := Create(conf, ml)
	if err != nil {
		t.Fatalf("unexpected err. %s", err)
	}

	// Create a second ring
	conf2 := fastConf()
	conf2.Hostname = "test2"
	r2, err := Join(conf2, ml, "test")
	if err != nil {
		t.Fatalf("failed to join local node! Got %s", err)
	}

	// Wait for some stabilization
	<-time.After(100 * time.Millisecond)

	// Try key lookup
	keys := [][]byte{[]byte("test"), []byte("foo"), []byte("bar")}
	for _, k := range keys {
		vn1, err := r.Lookup(3, k)
		if err != nil {
			t.Fatalf("unexpected err %s", err)
		}
		vn2, err := r2.Lookup(3, k)
		if err != nil {
			t.Fatalf("unexpected err %s", err)
		}
		if len(vn1) != len(vn2) {
			t.Fatalf("result len differs!")
		}
		for idx := range vn1 {
			if vn1[idx].String() != vn2[idx].String() {
				t.Fatalf("results differ!")
			}
		}
	}
}
