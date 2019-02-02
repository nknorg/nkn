package node

import (
	"encoding/hex"
	"encoding/json"

	"github.com/nknorg/nkn/util/log"
	nnetnode "github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay/chord"
)

type ChordRemoteNodeInfo nnetnode.RemoteNode

type ChordInfo struct {
	LocalNode    *LocalNode                     `json:"localNode"`
	Successors   []*ChordRemoteNodeInfo         `json:"successors"`
	Predecessors []*ChordRemoteNodeInfo         `json:"predecessors"`
	FingerTable  map[int][]*ChordRemoteNodeInfo `json:"fingerTable"`
}

func (localNode *LocalNode) GetNeighborInfo() []*RemoteNode {
	return localNode.GetNeighbors(nil)
}

func (rn *ChordRemoteNodeInfo) MarshalJSON() ([]byte, error) {
	var out map[string]interface{}

	buf, err := json.Marshal(rn.Node.Node)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(buf, &out)
	if err != nil {
		return nil, err
	}

	delete(out, "data")
	out["id"] = hex.EncodeToString(rn.Node.Node.Id)
	out["isOutBound"] = rn.IsOutbound

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
		successors = append(successors, (*ChordRemoteNodeInfo)(n))
	}

	for _, n := range c.Predecessors() {
		predecessors = append(predecessors, (*ChordRemoteNodeInfo)(n))
	}

	for i, nodes := range c.FingerTable() {
		if len(nodes) == 0 {
			continue
		}

		for _, n := range nodes {
			fingerTable[i] = append(fingerTable[i], (*ChordRemoteNodeInfo)(n))
		}
	}

	return &ChordInfo{
		LocalNode:    localNode,
		Successors:   successors,
		Predecessors: predecessors,
		FingerTable:  fingerTable,
	}
}
