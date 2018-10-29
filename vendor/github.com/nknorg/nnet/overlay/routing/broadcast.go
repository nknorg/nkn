package routing

import (
	"bytes"

	"github.com/nknorg/nnet/node"
)

const (
	// BroadcastRoutingNumWorkers determines how many concurrent goroutines are
	// handling broadcast messages
	BroadcastRoutingNumWorkers = 1
)

// BroadcastRouting is for message to all other nodes in the network
type BroadcastRouting struct {
	*Routing
	localNode *node.LocalNode
}

// NewBroadcastRouting creates a new BroadcastRouting
func NewBroadcastRouting(localMsgChan chan<- *node.RemoteMessage, rxMsgChan <-chan *node.RemoteMessage, localNode *node.LocalNode) (*BroadcastRouting, error) {
	r, err := NewRouting(localMsgChan, rxMsgChan)
	if err != nil {
		return nil, err
	}

	br := &BroadcastRouting{
		Routing:   r,
		localNode: localNode,
	}

	return br, nil
}

// Start starts handling broadcast message from rxChan
func (br *BroadcastRouting) Start() error {
	return br.Routing.Start(br, BroadcastRoutingNumWorkers)
}

// GetNodeToRoute returns the local node and remote nodes to route message to
func (br *BroadcastRouting) GetNodeToRoute(remoteMsg *node.RemoteMessage) (*node.LocalNode, []*node.RemoteNode, error) {
	localNode := br.localNode
	if remoteMsg.RemoteNode == nil {
		localNode = nil
	}

	nonSenderNeighbors, err := br.localNode.GetNeighbors(func(rn *node.RemoteNode) bool {
		return rn != remoteMsg.RemoteNode && !bytes.Equal(rn.Id, remoteMsg.Msg.SrcId)
	})
	if err != nil {
		return nil, nil, err
	}

	return localNode, nonSenderNeighbors, nil
}
