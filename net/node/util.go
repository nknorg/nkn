package node

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
)

// GenChordID generates an ID for the node
func GenChordID(host string) []byte {
	hash := sha256.New()
	hash.Write([]byte(host))
	return hash.Sum(nil)
}

func chordIDToNodeID(chordID []byte) (uint64, error) {
	var nodeID uint64
	err := binary.Read(bytes.NewBuffer(chordID), binary.LittleEndian, &nodeID)
	return nodeID, err
}
