package node

import (
	"crypto/sha256"
	"encoding/hex"
)

// GenChordID generates an ID for the node
func GenChordID(host string) []byte {
	hash := sha256.New()
	hash.Write([]byte(host))
	return hash.Sum(nil)
}

func chordIDToNodeID(chordID []byte) string {
	return hex.EncodeToString(chordID)
}
