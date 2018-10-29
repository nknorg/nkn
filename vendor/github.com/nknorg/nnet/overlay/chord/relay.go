package chord

import (
	"errors"

	"github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay/routing"
)

const (
	// RelayRoutingNumWorkers determines how many concurrent goroutines are
	// handling relay messages
	RelayRoutingNumWorkers = 1
)

// RelayRouting is for message from a remote node to another remote node or
// local node
type RelayRouting struct {
	*routing.Routing
	chord *Chord
}

// NewRelayRouting creates a new RelayRouting
func NewRelayRouting(localMsgChan chan<- *node.RemoteMessage, rxMsgChan <-chan *node.RemoteMessage, chord *Chord) (*RelayRouting, error) {
	r, err := routing.NewRouting(localMsgChan, rxMsgChan)
	if err != nil {
		return nil, err
	}

	rr := &RelayRouting{
		Routing: r,
		chord:   chord,
	}

	return rr, nil
}

// Start starts handling relay message from rxChan
func (rr *RelayRouting) Start() error {
	return rr.Routing.Start(rr, RelayRoutingNumWorkers)
}

// GetNodeToRoute returns the local node and remote nodes to route message to
func (rr *RelayRouting) GetNodeToRoute(remoteMsg *node.RemoteMessage) (*node.LocalNode, []*node.RemoteNode, error) {
	succ := rr.chord.successors.GetFirst()
	if succ == nil {
		return nil, nil, errors.New("Local node has no successor yet")
	}

	if betweenLeftIncl(rr.chord.LocalNode.Id, succ.Id, remoteMsg.Msg.DestId) {
		return rr.chord.LocalNode, nil, nil
	}

	nextHop := succ
	minDist := distance(succ.Id, remoteMsg.Msg.DestId, rr.chord.nodeIDBits)

	for _, remoteNode := range rr.chord.successors.ToRemoteNodeList(false) {
		if remoteNode == remoteMsg.RemoteNode {
			continue
		}
		dist := distance(remoteNode.Id, remoteMsg.Msg.DestId, rr.chord.nodeIDBits)
		if dist.Cmp(minDist) < 0 {
			nextHop = remoteNode
			minDist = dist
		}
	}

	for _, finger := range rr.chord.fingerTable {
		for _, remoteNode := range finger.ToRemoteNodeList(false) {
			if remoteNode == remoteMsg.RemoteNode {
				continue
			}
			dist := distance(remoteNode.Id, remoteMsg.Msg.DestId, rr.chord.nodeIDBits)
			if dist.Cmp(minDist) < 0 {
				nextHop = remoteNode
				minDist = dist
			}
		}
	}

	return nil, []*node.RemoteNode{nextHop}, nil
}
