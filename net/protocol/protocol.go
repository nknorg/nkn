package protocol

import (
	"bytes"
	"encoding/binary"

	"github.com/nknorg/nkn/pb"
)

type ChordInfo struct {
	Node         ChordNodeInfo
	Successors   []*ChordNodeInfo
	Predecessors []*ChordNodeInfo
	FingerTable  map[int][]*ChordNodeInfo
}

type ChordNodeInfo struct {
	ID         string
	Addr       string
	IsOutbound bool
	pb.NodeData
}

type NodeAddr struct {
	Time    int64
	IpAddr  [16]byte
	IpStr   string
	InOut   string
	Port    uint16
	ID      uint64
	NKNaddr string
}

func (msg *NodeAddr) Deserialization(p []byte) error {
	buf := bytes.NewBuffer(p)
	err := binary.Read(buf, binary.LittleEndian, msg)
	return err
}

func (msg NodeAddr) Serialization() ([]byte, error) {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.LittleEndian, msg)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), err
}
