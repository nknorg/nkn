package routing

import (
	"errors"

	"github.com/nknorg/nnet/node"
)

const (
	// DirectRoutingNumWorkers determines how many concurrent goroutines are
	// handling direct messages
	DirectRoutingNumWorkers = 1
)

// DirectRouting is for message to local node directly from remote node
type DirectRouting struct {
	*Routing
}

// NewDirectRouting creates a new DirectRouting
func NewDirectRouting(localMsgChan chan<- *node.RemoteMessage, rxMsgChan <-chan *node.RemoteMessage) (*DirectRouting, error) {
	r, err := NewRouting(localMsgChan, rxMsgChan)
	if err != nil {
		return nil, err
	}

	dr := &DirectRouting{
		Routing: r,
	}

	return dr, nil
}

// Start starts handling direct message from rxChan
func (dr *DirectRouting) Start() error {
	return dr.Routing.Start(dr, DirectRoutingNumWorkers)
}

// GetNodeToRoute returns the local node and remote nodes to route message to
func (dr *DirectRouting) GetNodeToRoute(remoteMsg *node.RemoteMessage) (*node.LocalNode, []*node.RemoteNode, error) {
	if remoteMsg.RemoteNode == nil {
		return nil, nil, errors.New("Message is sent by local node")
	}

	return remoteMsg.RemoteNode.LocalNode, nil, nil
}
