/*
This package is used to provide an implementation of the
Chord network protocol.
*/
package chord

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"net"
	"strconv"
	"time"

	"github.com/nknorg/nkn/api/httpjson/client"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

// Implements the methods needed for a Chord ring
type Transport interface {
	// Gets a list of the vnodes on the box
	ListVnodes(string) ([]*Vnode, error)

	// Ping a Vnode, check for liveness
	Ping(*Vnode) (bool, error)

	// Request a nodes predecessor
	GetPredecessor(*Vnode) (*Vnode, error)

	// Notify our successor of ourselves
	Notify(target, self *Vnode) ([]*Vnode, error)

	// Find a successor
	FindSuccessors(*Vnode, int, []byte) ([]*Vnode, error)

	// Clears a predecessor if it matches a given vnode. Used to leave.
	ClearPredecessor(target, self *Vnode) error

	// Instructs a node to skip a given successor. Used to leave.
	SkipSuccessor(target, self *Vnode) error

	// Register for an RPC callbacks
	Register(*Vnode, VnodeRPC)
}

// These are the methods to invoke on the registered vnodes
type VnodeRPC interface {
	GetPredecessor() (*Vnode, error)
	Notify(*Vnode) ([]*Vnode, error)
	FindSuccessors(int, []byte) ([]*Vnode, error)
	ClearPredecessor(*Vnode) error
	SkipSuccessor(*Vnode) error
}

// Delegate to notify on ring events
type Delegate interface {
	NewPredecessor(local, remoteNew, remotePrev *Vnode)
	Leaving(local, pred, succ *Vnode)
	PredecessorLeaving(local, remote *Vnode)
	SuccessorLeaving(local, remote *Vnode)
	Shutdown()
}

// Configuration for Chord nodes
type Config struct {
	Hostname      string           // Local host name
	NumVnodes     int              // Number of vnodes per physical node
	HashFunc      func() hash.Hash // Hash function to use
	StabilizeMin  time.Duration    // Minimum stabilization time
	StabilizeMax  time.Duration    // Maximum stabilization time
	NumSuccessors int              // Number of successors to maintain
	Delegate      Delegate         // Invoked to handle ring events
	hashBits      int              // Bit size of the hash function
	SeedNodeAddr  string           // Join a ring via a seed node
	JoinBlkHeight uint32           // Current BlockHeight when join ring
}

// configData : Data of Config struct for json.Marshal in API
type configData struct {
	Hostname      string // Local host name
	NumVnodes     int    // Number of vnodes per physical node
	NumSuccessors int    // Number of successors to maintain
	HashBits      int    // Bit size of the hash function
	SeedNodeAddr  string // Join a ring via a seed node
	JoinBlkHeight uint32 // Current BlockHeight when join ring
}

// Represents an Vnode, local or remote
type Vnode struct {
	Id         []byte // Virtual ID
	Host       string // Chord Host identifier
	NodePort   uint16 // Node port
	HttpWsPort uint16 // Websocket port
}

type VnodeData struct {
	Id         string // Virtual ID of Hex String
	Host       string // Chord Host identifier
	NodePort   uint16 // Node port
	HttpWsPort uint16 // Websocket port
}

// Represents a local Vnode
type localVnode struct {
	Vnode
	ring           *Ring
	successors     []*Vnode
	finger         []*Vnode
	last_finger    int
	predecessor    *Vnode
	stabilized     time.Time
	timer          *time.Timer
	OnNewSuccessor func()
}

// localVnodeData : Data of localVnode for json.Marshal in API
type localVnodeData struct {
	VnodeData
	Successors  []*VnodeData
	Finger      []*VnodeData
	Predecessor *VnodeData
	Last_finger int
}

// Stores the state required for a Chord ring
type Ring struct {
	config     *Config
	transport  Transport
	Vnodes     []*localVnode
	delegateCh chan func()
	shutdown   chan bool
}

// RingData : Data of Ring for json.Marshal in API
type RingData struct {
	Conf   *configData
	Vnodes []*localVnodeData
}

var ring *Ring

// Returns the default Ring configuration
func DefaultConfig(hostname string) *Config {
	return &Config{
		Hostname:      hostname,
		NumVnodes:     1,          // 1 vnodes
		HashFunc:      sha256.New, // SHA256
		StabilizeMin:  time.Duration(500 * time.Millisecond),
		StabilizeMax:  time.Duration(1500 * time.Millisecond),
		NumSuccessors: 16,  // 16 successors
		Delegate:      nil, // No delegate
		hashBits:      256, // 256bit hash function
	}
}

// Extract marshalable data from Config struct
func (c *Config) toData() *configData {
	return &configData{
		c.Hostname,
		c.NumVnodes,
		c.NumSuccessors,
		c.hashBits,
		c.SeedNodeAddr,
		c.JoinBlkHeight,
	}
}

// Creates a new Chord ring given the config and transport
func Create(conf *Config, trans Transport) (*Ring, error) {
	// Initialize the hash bits
	conf.hashBits = conf.HashFunc().Size() * 8

	if conf.NumVnodes < conf.NumSuccessors+1 {
		conf.NumVnodes = conf.NumSuccessors + 1
	}

	// Create and initialize a ring
	ring = &Ring{}
	ring.init(conf, trans)
	ring.setLocalSuccessors()
	ring.schedule()
	return ring, nil
}

// Joins an existing Chord ring
func Join(conf *Config, trans Transport, existing string) (*Ring, error) {
	// Initialize the hash bits
	conf.hashBits = conf.HashFunc().Size() * 8

	// Request a list of Vnodes from the remote host
	hosts, err := trans.ListVnodes(existing)
	if err != nil {
		return nil, err
	}
	if hosts == nil || len(hosts) == 0 {
		return nil, fmt.Errorf("Remote host has no vnodes!")
	}

	// Create a ring
	ring = &Ring{}
	ring.init(conf, trans)

	// Acquire a live successor for each Vnode
	for _, vn := range ring.Vnodes {
		// Get the nearest remote vnode
		nearest := nearestVnodeToKey(hosts, vn.Id)

		// Query for a list of successors to this Vnode
		succs, err := trans.FindSuccessors(nearest, conf.NumSuccessors, vn.Id)
		if err != nil {
			return nil, fmt.Errorf("Failed to find successor for vnodes! Got %s", err)
		}
		if succs == nil || len(succs) == 0 {
			return nil, fmt.Errorf("Failed to find successor for vnodes! Got no vnodes!")
		}

		// Assign the successors
		skip := 0
		for idx, s := range succs {
			if s != nil && s.String() != vn.String() {
				vn.successors[idx-skip] = s
			} else {
				skip++
			}
		}
	}

	// Start delegate handler
	if ring.config.Delegate != nil {
		go ring.delegateHandler()
	}

	// Do a fast stabilization, will schedule regular execution
	for _, vn := range ring.Vnodes {
		vn.stabilize()
	}
	return ring, nil
}

// Leaves a given Chord ring and shuts down the local vnodes
func (r *Ring) Leave() error {
	// Shutdown the vnodes first to avoid further stabilization runs
	r.stopVnodes()

	// Instruct each vnode to leave
	var err error
	for _, vn := range r.Vnodes {
		err = mergeErrors(err, vn.leave())
	}

	// Wait for the delegate callbacks to complete
	r.stopDelegate()
	return err
}

// Shutdown shuts down the local processes in a given Chord ring
// Blocks until all the vnodes terminate.
func (r *Ring) Shutdown() {
	r.stopVnodes()
	r.stopDelegate()
}

// Does a key lookup for up to N successors of a key
func (r *Ring) Lookup(n int, key []byte) ([]*Vnode, error) {
	// Ensure that n is sane
	if n > r.config.NumSuccessors {
		return nil, fmt.Errorf("Cannot ask for more successors than NumSuccessors!")
	}

	// Hash the key
	h := r.config.HashFunc()
	h.Write(key)
	key_hash := h.Sum(nil)

	// Find the nearest local vnode
	nearest := r.nearestVnode(key_hash)

	// Use the nearest node for the lookup
	successors, err := nearest.FindSuccessors(n, key_hash)
	if err != nil {
		return nil, err
	}

	// Trim the nil successors
	for successors[len(successors)-1] == nil {
		successors = successors[:len(successors)-1]
	}
	return successors, nil
}

// Ring create and join functions
func prepRing(port uint16) (*Config, *TCPTransport, error) {
	hostname := fmt.Sprintf("%s:%d", config.Parameters.Hostname, port)
	listen := fmt.Sprintf(":%d", port)
	conf := DefaultConfig(hostname)
	timeout := time.Duration(3 * time.Second)
	trans, err := InitTCPTransport(listen, timeout)
	if err != nil {
		return nil, nil, err
	}
	return conf, trans, nil
}

// Creat the ring
func CreateNet() (*Ring, *TCPTransport, error) {
	log.Debug()
	c, t, err := prepRing(config.Parameters.ChordPort)
	if err != nil {
		log.Errorf("unexpected err. %s", err)
		return nil, nil, err
	}

	// Create initial ring
	r, err := Create(c, t)
	if err != nil {
		log.Errorf("unexpected err. %s", err)
		return nil, nil, err
	}

	return r, t, nil
}

// Join the ring
func JoinNet() (*Ring, *TCPTransport, error) {
	log.Debug()
	c, t, err := prepRing(config.Parameters.ChordPort)
	if err != nil {
		log.Errorf("unexpected err. %s", err)
		return nil, nil, err
	}

	for _, seed := range config.Parameters.SeedList {
		info, err := client.GetNodeState(seed)
		if err != nil {
			log.Warnf("Can't get remote node info from [%s]", seed)
			continue
		}

		c.JoinBlkHeight = info.Height
		c.SeedNodeAddr = net.JoinHostPort(info.Addr, strconv.Itoa(int(info.ChordPort)))

		// Join ring
		r, err := Join(c, t, c.SeedNodeAddr)
		if err == nil {
			log.Infof("Join ring by Seed [%s] successful", seed)
			return r, t, nil
		}
		log.Errorf("failed to join [%s] local node! Got %s", c.SeedNodeAddr, err)
	}

	return nil, nil, fmt.Errorf("Tried all Seeds but still failed")
}

func GetRing() *Ring {
	return ring
}
