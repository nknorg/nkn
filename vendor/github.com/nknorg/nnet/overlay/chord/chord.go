package chord

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nnet/config"
	"github.com/nknorg/nnet/log"
	"github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay"
	"github.com/nknorg/nnet/overlay/routing"
	"github.com/nknorg/nnet/protobuf"
)

const (
	// How many concurrent goroutines are handling messages
	numWorkers = 1
)

// Chord is the overlay network based on Chord DHT
type Chord struct {
	*overlay.Overlay
	nodeIDBits            uint32
	minNumSuccessors      uint32
	numSuccessorsFactor   uint32
	baseStabilizeInterval time.Duration
	successors            *NeighborList
	predecessors          *NeighborList
	fingerTable           []*NeighborList
	neighbors             *NeighborList
	*middlewareStore
}

// NewChord creates a Chord overlay network
func NewChord(localNode *node.LocalNode, conf *config.Config) (*Chord, error) {
	ovl, err := overlay.NewOverlay(localNode)
	if err != nil {
		return nil, err
	}

	nodeIDBits := conf.NodeIDBytes * 8

	next := nextID(localNode.Id, nodeIDBits)
	prev := prevID(localNode.Id, nodeIDBits)

	successors, err := NewNeighborList(next, prev, nodeIDBits, conf.MinNumSuccessors, false)
	if err != nil {
		return nil, err
	}

	predecessors, err := NewNeighborList(prev, next, nodeIDBits, conf.MinNumSuccessors, true)
	if err != nil {
		return nil, err
	}

	fingerTable := make([]*NeighborList, nodeIDBits)
	for i := uint32(0); i < nodeIDBits; i++ {
		startID := powerOffset(localNode.Id, i, nodeIDBits)
		endID := prevID(powerOffset(localNode.Id, i+1, nodeIDBits), nodeIDBits)
		fingerTable[i], err = NewNeighborList(startID, endID, nodeIDBits, conf.NumFingerSuccessors, false)
		if err != nil {
			return nil, err
		}
	}

	neighbors, err := NewNeighborList(next, prev, nodeIDBits, 0, false)
	if err != nil {
		return nil, err
	}

	middlewareStore := newMiddlewareStore()

	c := &Chord{
		Overlay:               ovl,
		nodeIDBits:            nodeIDBits,
		minNumSuccessors:      conf.MinNumSuccessors,
		numSuccessorsFactor:   conf.NumSuccessorsFactor,
		baseStabilizeInterval: conf.BaseStabilizeInterval,
		successors:            successors,
		predecessors:          predecessors,
		fingerTable:           fingerTable,
		neighbors:             neighbors,
		middlewareStore:       middlewareStore,
	}

	directRxMsgChan, err := localNode.GetRxMsgChan(protobuf.DIRECT)
	if err != nil {
		return nil, err
	}
	directRouting, err := routing.NewDirectRouting(ovl.LocalMsgChan, directRxMsgChan)
	if err != nil {
		return nil, err
	}
	err = ovl.AddRouter(protobuf.DIRECT, directRouting)
	if err != nil {
		return nil, err
	}

	relayRxMsgChan, err := localNode.GetRxMsgChan(protobuf.RELAY)
	if err != nil {
		return nil, err
	}
	relayRouting, err := NewRelayRouting(ovl.LocalMsgChan, relayRxMsgChan, c)
	if err != nil {
		return nil, err
	}
	err = ovl.AddRouter(protobuf.RELAY, relayRouting)
	if err != nil {
		return nil, err
	}

	broadcastRxMsgChan, err := localNode.GetRxMsgChan(protobuf.BROADCAST_PUSH)
	if err != nil {
		return nil, err
	}
	broadcastRouting, err := routing.NewBroadcastRouting(ovl.LocalMsgChan, broadcastRxMsgChan, localNode)
	if err != nil {
		return nil, err
	}
	err = ovl.AddRouter(protobuf.BROADCAST_PUSH, broadcastRouting)
	if err != nil {
		return nil, err
	}

	broadcastTreeRxMsgChan, err := localNode.GetRxMsgChan(protobuf.BROADCAST_TREE)
	if err != nil {
		return nil, err
	}
	broadcastTreeRouting, err := NewBroadcastTreeRouting(ovl.LocalMsgChan, broadcastTreeRxMsgChan, c)
	if err != nil {
		return nil, err
	}
	err = ovl.AddRouter(protobuf.BROADCAST_TREE, broadcastTreeRouting)
	if err != nil {
		return nil, err
	}

	err = localNode.ApplyMiddleware(node.RemoteNodeReady(func(rn *node.RemoteNode) bool {
		c.addRemoteNode(rn)
		return true
	}))
	if err != nil {
		return nil, err
	}

	err = localNode.ApplyMiddleware(node.RemoteNodeDisconnected(func(rn *node.RemoteNode) bool {
		c.removeNeighbor(rn)
		return true
	}))
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Start starts the runtime loop of the chord network
func (c *Chord) Start() error {
	c.StartOnce.Do(func() {
		var joinOnce sync.Once

		err := c.ApplyMiddleware(SuccessorAdded(func(remoteNode *node.RemoteNode, index int) bool {
			joinOnce.Do(func() {
				// prev is used to prevent msg being routed to self
				prev := prevID(c.LocalNode.Id, c.nodeIDBits)
				succs, err := c.FindSuccessors(prev, c.successors.Cap())
				if err != nil {
					c.Stop(fmt.Errorf("Join failed: %s", err))
					return
				}

				for _, succ := range succs {
					if CompareID(succ.Id, c.LocalNode.Id) != 0 {
						err = c.Connect(succ.Addr, succ.Id)
						if err != nil {
							log.Error(err)
						}
					}
				}

				c.stabilize()
			})
			return true
		}))
		if err != nil {
			c.Stop(err)
			return
		}

		for _, f := range c.middlewareStore.networkWillStart {
			if !f(c) {
				break
			}
		}

		err = c.StartRouters()
		if err != nil {
			c.Stop(err)
			return
		}

		for i := 0; i < numWorkers; i++ {
			go c.handleMsg()
		}

		err = c.LocalNode.Start()
		if err != nil {
			c.Stop(err)
			return
		}

		for _, f := range c.middlewareStore.networkStarted {
			if !f(c) {
				break
			}
		}
	})

	return nil
}

// Stop stops the chord network
func (c *Chord) Stop(err error) {
	c.StopOnce.Do(func() {
		for _, f := range c.middlewareStore.networkWillStop {
			if !f(c) {
				break
			}
		}

		if err != nil {
			log.Warningf("Chord overlay stops because of error: %s", err)
		} else {
			log.Infof("Chord overlay stops")
		}

		for _, remoteNode := range c.neighbors.ToRemoteNodeList(false) {
			go remoteNode.Stop(err)
		}

		c.LocalNode.Stop(err)

		c.LifeCycle.Stop()

		c.StopRouters(err)

		for _, f := range c.middlewareStore.networkStopped {
			if !f(c) {
				break
			}
		}
	})
}

// Join joins an existing chord network starting from the seedNodeAddr
func (c *Chord) Join(seedNodeAddr string) error {
	err := c.Connect(seedNodeAddr, nil)
	if err != nil {
		return err
	}

	return nil
}

// handleMsg starts a loop that handles received msg
func (c *Chord) handleMsg() {
	var remoteMsg *node.RemoteMessage
	var shouldLocalNodeHandleMsg bool
	var err error

	for {
		if c.IsStopped() {
			return
		}

		remoteMsg = <-c.LocalMsgChan

		shouldLocalNodeHandleMsg, err = c.handleRemoteMessage(remoteMsg)
		if err != nil {
			log.Error(err)
			continue
		}

		if shouldLocalNodeHandleMsg {
			err = c.LocalNode.HandleRemoteMessage(remoteMsg)
			if err != nil {
				log.Error(err)
				continue
			}
		}
	}
}

// stabilize periodically updates successors, predecessors and fingerTable to
// keep topology correct
func (c *Chord) stabilize() {
	go c.updateSuccessors()
	go c.updatePredecessors()
	go c.updateFinger()
	go c.findNewPredecessors()
	go c.findNewFinger()
}

// updateSuccessors periodically updates successors
func (c *Chord) updateSuccessors() {
	var err error

	for {
		if c.IsStopped() {
			return
		}

		time.Sleep(randDuration(c.baseStabilizeInterval))

		err = c.updateNeighborList(c.successors)
		if err != nil {
			log.Error("Update successors error:", err)
		}
	}
}

// updatePredecessors periodically updates predecessors
func (c *Chord) updatePredecessors() {
	var err error

	for {
		if c.IsStopped() {
			return
		}

		time.Sleep(3 * randDuration(c.baseStabilizeInterval))

		err = c.updateNeighborList(c.predecessors)
		if err != nil {
			log.Error("Update predecessor error:", err)
		}
	}
}

// findNewPredecessors periodically find new predecessors
func (c *Chord) findNewPredecessors() {
	var err error
	var existing *node.RemoteNode
	var maybeNewNodes []*protobuf.Node

	for {
		if c.IsStopped() {
			return
		}

		time.Sleep(3 * randDuration(c.baseStabilizeInterval))

		maybeNewNodes, err = c.FindPredecessors(c.predecessors.startID, 1)
		if err != nil {
			log.Error("Find predecessors error:", err)
			continue
		}

		for _, n := range maybeNewNodes {
			if c.predecessors.IsIDInRange(n.Id) && !c.predecessors.Exists(n.Id) {
				existing = c.predecessors.GetFirst()
				if existing == nil || c.predecessors.cmp(n, existing.Node.Node) < 0 {
					err = c.Connect(n.Addr, n.Id)
					if err != nil {
						log.Error("Connect to new predecessor error:", err)
					}
				}
			}
		}
	}
}

// updateFinger periodically updates non-empty finger table items
func (c *Chord) updateFinger() {
	var err error
	var finger *NeighborList

	for {
		for _, finger = range c.fingerTable {
			if finger.IsEmpty() {
				continue
			}

			if c.IsStopped() {
				return
			}

			time.Sleep(randDuration(c.baseStabilizeInterval))

			err = c.updateNeighborList(finger)
			if err != nil {
				log.Error("Update finger table error:", err)
			}
		}

		// to prevent endless looping when fingerTable is all empty
		time.Sleep(randDuration(c.baseStabilizeInterval))
	}
}

// findNewFinger periodically find new finger table node
func (c *Chord) findNewFinger() {
	var err error
	var i int
	var existing *node.RemoteNode
	var succs []*protobuf.Node

	for {
		for i = 0; i < len(c.fingerTable); i++ {
			if c.IsStopped() {
				return
			}

			time.Sleep(randDuration(c.baseStabilizeInterval))

			succs, err = c.FindSuccessors(c.fingerTable[i].startID, 1)
			if err != nil {
				log.Error("Find successor for finger table error:", err)
				continue
			}

			if len(succs) == 0 {
				continue
			}

			for i < len(c.fingerTable) {
				if c.fingerTable[i].IsIDInRange(succs[0].Id) && !c.fingerTable[i].Exists(succs[0].Id) {
					existing = c.fingerTable[i].GetFirst()
					if existing == nil || c.fingerTable[i].cmp(succs[0], existing.Node.Node) < 0 {
						err = c.Connect(succs[0].Addr, succs[0].Id)
						if err != nil {
							log.Error("Connect to new successor error:", err)
						}
					}
					break
				}
				i++
			}
		}
	}
}

// updateSuccPredLen updates the length of successors and predecessors according
// to the number of non empty finger table
func (c *Chord) updateSuccPredMaxNumNodes() {
	numNonEmptyFinger := 0
	for _, finger := range c.fingerTable {
		if !finger.IsEmpty() {
			numNonEmptyFinger++
		}
	}

	succPredLen := c.numSuccessorsFactor * uint32(numNonEmptyFinger)

	if succPredLen > c.minNumSuccessors {
		c.successors.SetMaxNumNodes(succPredLen)
		c.predecessors.SetMaxNumNodes(succPredLen)
	}
}

// GetSuccAndPred sends a GetSuccAndPred message to remote node and returns its
// successors and predecessor if no error occured
func GetSuccAndPred(remoteNode *node.RemoteNode, numSucc, numPred uint32) ([]*protobuf.Node, []*protobuf.Node, error) {
	msg, err := NewGetSuccAndPredMessage(numSucc, numPred)
	if err != nil {
		return nil, nil, err
	}

	reply, err := remoteNode.SendMessageSync(msg)
	if err != nil {
		return nil, nil, err
	}

	replyBody := &protobuf.GetSuccAndPredReply{}
	err = proto.Unmarshal(reply.Msg.Message, replyBody)
	if err != nil {
		return nil, nil, err
	}

	return replyBody.Successors, replyBody.Predecessors, nil
}

// FindSuccAndPred sends a FindSuccAndPred message and returns numSucc
// successors and numPred predecessors of a given key id
func (c *Chord) FindSuccAndPred(key []byte, numSucc, numPred uint32) ([]*protobuf.Node, []*protobuf.Node, error) {
	succ := c.successors.GetFirst()
	if succ == nil {
		return nil, nil, errors.New("Local node has no successor yet")
	}

	if CompareID(key, c.LocalNode.Id) == 0 || between(c.LocalNode.Id, succ.Id, key) {
		var succs, preds []*protobuf.Node

		if numSucc > 0 {
			if CompareID(key, c.LocalNode.Id) == 0 {
				succs = append(succs, c.LocalNode.Node.Node)
			}

			succs = append(succs, c.successors.ToProtoNodeList(true)...)

			if succs != nil && len(succs) > int(numSucc) {
				succs = succs[:numSucc]
			}
		}

		if numPred > 0 {
			preds = []*protobuf.Node{c.LocalNode.Node.Node}
			preds = append(preds, c.predecessors.ToProtoNodeList(true)...)

			if preds != nil && len(preds) > int(numPred) {
				preds = preds[:numPred]
			}
		}

		return succs, preds, nil
	}

	msg, err := NewFindSuccAndPredMessage(key, numSucc, numPred)
	if err != nil {
		return nil, nil, err
	}

	reply, _, err := c.SendMessageSync(msg, protobuf.RELAY)
	if err != nil {
		return nil, nil, err
	}

	replyBody := &protobuf.FindSuccAndPredReply{}
	err = proto.Unmarshal(reply.Message, replyBody)
	if err != nil {
		return nil, nil, err
	}

	if len(replyBody.Successors) > int(numSucc) {
		replyBody.Successors = replyBody.Successors[:numSucc]
	}

	if len(replyBody.Predecessors) > int(numPred) {
		replyBody.Predecessors = replyBody.Predecessors[:numPred]
	}

	return replyBody.Successors, replyBody.Predecessors, nil
}

// FindSuccessors sends a FindSuccessors message and returns numSucc successors
// of a given key id
func (c *Chord) FindSuccessors(key []byte, numSucc uint32) ([]*protobuf.Node, error) {
	succs, _, err := c.FindSuccAndPred(key, numSucc, 0)
	return succs, err
}

// FindPredecessors sends a FindPredecessors message and returns numPred
// predecessors of a given key id
func (c *Chord) FindPredecessors(key []byte, numPred uint32) ([]*protobuf.Node, error) {
	_, preds, err := c.FindSuccAndPred(key, 0, numPred)
	return preds, err
}
