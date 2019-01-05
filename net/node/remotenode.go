package node

import (
	"errors"
	"net"
	"net/url"
	"strconv"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nnet/log"
	nnetnode "github.com/nknorg/nnet/node"
)

type RemoteNode struct {
	*Node
	localNode     *LocalNode
	nnetNode      *nnetnode.RemoteNode
	flightHeights []uint32
}

func NewRemoteNode(localNode *LocalNode, nnetNode *nnetnode.RemoteNode) (*RemoteNode, error) {
	var nodeData pb.NodeData
	err := proto.Unmarshal(nnetNode.Node.Data, &nodeData)
	if err != nil {
		return nil, err
	}

	publicKey, err := crypto.DecodePoint(nodeData.PublicKey)
	if err != nil {
		return nil, err
	}

	node, err := NewNode(nnetNode.Id, publicKey, nodeData)
	if err != nil {
		return nil, err
	}

	remoteNode := &RemoteNode{
		Node:      node,
		localNode: localNode,
		nnetNode:  nnetNode,
	}

	return remoteNode, nil
}

func (remoteNode *RemoteNode) LocalNode() *LocalNode {
	return remoteNode.localNode
}

func (remoteNode *RemoteNode) GetAddrStr() string {
	return remoteNode.nnetNode.Addr
}

func (remoteNode *RemoteNode) GetAddr() string {
	address, _ := url.Parse(remoteNode.GetAddrStr())
	return address.Hostname()
}

func (remoteNode *RemoteNode) GetAddr16() ([16]byte, error) {
	var result [16]byte
	ip := net.ParseIP(remoteNode.GetAddr()).To16()
	if ip == nil {
		log.Error("Parse IP address error\n")
		return result, errors.New("Parse IP address error")
	}

	copy(result[:], ip[:16])
	return result, nil
}

func (remoteNode *RemoteNode) GetPort() uint16 {
	address, _ := url.Parse(remoteNode.GetAddrStr())
	port, _ := strconv.Atoi(address.Port())
	return uint16(port)
}

func (remoteNode *RemoteNode) GetConnDirection() string {
	if remoteNode.nnetNode.IsOutbound {
		return "Outbound"
	} else {
		return "Inbound"
	}
}

func (remoteNode *RemoteNode) GetChordAddr() []byte {
	return remoteNode.nnetNode.Id
}

func (remoteNode *RemoteNode) CloseConn() {
	remoteNode.nnetNode.Stop(nil)
}
