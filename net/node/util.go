package node

import (
	"encoding/hex"
)

func chordIDToNodeID(chordID []byte) string {
	return hex.EncodeToString(chordID)
}
