package chord

import (
	"bytes"

	"github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay/routing"
)

const (
	// BroadcastTreeRoutingNumWorkers determines how many concurrent goroutines
	// are handling broadcast tree messages
	BroadcastTreeRoutingNumWorkers = 1
)

// BroadcastTreeRouting is for message from a remote node to all other nodes in
// the network following spanning tree
type BroadcastTreeRouting struct {
	*routing.Routing
	chord *Chord
}

// NewBroadcastTreeRouting creates a new BroadcastTreeRouting
func NewBroadcastTreeRouting(localMsgChan chan<- *node.RemoteMessage, rxMsgChan <-chan *node.RemoteMessage, chord *Chord) (*BroadcastTreeRouting, error) {
	r, err := routing.NewRouting(localMsgChan, rxMsgChan)
	if err != nil {
		return nil, err
	}

	btr := &BroadcastTreeRouting{
		Routing: r,
		chord:   chord,
	}

	return btr, nil
}

// Start starts handling broadcast tree message from rxChan
func (btr *BroadcastTreeRouting) Start() error {
	return btr.Routing.Start(btr, BroadcastTreeRoutingNumWorkers)
}

// GetNodeToRoute returns the local node and remote nodes to route message to
func (btr *BroadcastTreeRouting) GetNodeToRoute(remoteMsg *node.RemoteMessage) (*node.LocalNode, []*node.RemoteNode, error) {
	var localNode *node.LocalNode
	remoteNodes := make([]*node.RemoteNode, 0)
	maxIdx := len(btr.chord.fingerTable)

	if remoteMsg.RemoteNode != nil {
		localNode = btr.chord.LocalNode
		dist := distance(remoteMsg.RemoteNode.Id, btr.chord.LocalNode.Id, btr.chord.nodeIDBits)
		maxIdx = dist.BitLen() - 1
	}

	for i := 0; i < maxIdx; i++ {
		for _, remoteNode := range btr.chord.fingerTable[i].ToRemoteNodeList(false) {
			if remoteNode != remoteMsg.RemoteNode && !bytes.Equal(remoteNode.Id, remoteMsg.Msg.SrcId) {
				remoteNodes = append(remoteNodes, remoteNode)
			}
		}
	}

	return localNode, remoteNodes, nil
}
