package node

import (
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/util/log"
	nnetnode "github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay/chord"
)

type ChordRemoteNodeInfo struct {
	localNode  *LocalNode
	remoteNode *nnetnode.RemoteNode
}

func newChordRemoteNodeInfo(localNode *LocalNode, remoteNode *nnetnode.RemoteNode) *ChordRemoteNodeInfo {
	return &ChordRemoteNodeInfo{
		localNode:  localNode,
		remoteNode: remoteNode,
	}
}

type ChordInfo struct {
	LocalNode    *LocalNode                     `json:"localNode"`
	Successors   []*ChordRemoteNodeInfo         `json:"successors"`
	Predecessors []*ChordRemoteNodeInfo         `json:"predecessors"`
	FingerTable  map[int][]*ChordRemoteNodeInfo `json:"fingerTable"`
}

func (localNode *LocalNode) GetNeighborInfo() []*RemoteNode {
	return localNode.GetNeighbors(nil)
}

func (crn *ChordRemoteNodeInfo) MarshalJSON() ([]byte, error) {
	var out map[string]interface{}

	buf, err := json.Marshal(crn.remoteNode.Node.Node)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(buf, &out)
	if err != nil {
		return nil, err
	}

	nodeData := &pb.NodeData{}
	err = proto.Unmarshal(crn.remoteNode.Node.Node.Data, nodeData)
	if err != nil {
		return nil, err
	}

	delete(out, "data")
	out["id"] = hex.EncodeToString(crn.remoteNode.Node.Node.Id)
	out["isOutbound"] = crn.remoteNode.IsOutbound
	out["roundTripTime"] = crn.remoteNode.GetRoundTripTime() / time.Millisecond
	out["protocolVersion"] = nodeData.ProtocolVersion

	out["connTime"] = 0
	nbr := crn.localNode.getNeighborByNNetNode(crn.remoteNode)
	if nbr != nil {
		out["connTime"] = time.Since(nbr.Node.startTime) / time.Second
	}

	return json.Marshal(out)
}

func (localNode *LocalNode) GetChordInfo() *ChordInfo {
	c, ok := localNode.nnet.Network.(*chord.Chord)
	if !ok {
		log.Errorf("Overlay is not chord")
		return nil
	}

	var successors, predecessors []*ChordRemoteNodeInfo
	fingerTable := make(map[int][]*ChordRemoteNodeInfo)

	for _, n := range c.Successors() {
		successors = append(successors, newChordRemoteNodeInfo(localNode, n))
	}

	for _, n := range c.Predecessors() {
		predecessors = append(predecessors, newChordRemoteNodeInfo(localNode, n))
	}

	for i, nodes := range c.FingerTable() {
		if len(nodes) == 0 {
			continue
		}

		for _, n := range nodes {
			fingerTable[i] = append(fingerTable[i], newChordRemoteNodeInfo(localNode, n))
		}
	}

	return &ChordInfo{
		LocalNode:    localNode,
		Successors:   successors,
		Predecessors: predecessors,
		FingerTable:  fingerTable,
	}
}
