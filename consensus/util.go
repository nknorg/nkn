package consensus

import (
	"encoding/binary"

	"github.com/nknorg/nkn/common"
)

// heiheightToKey convert block height to byte array
func heightToKey(height uint32) []byte {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, height)
	return key
}

func hasDuplicateHash(txnsHash []common.Uint256) bool {
	count := make(map[common.Uint256]int, len(txnsHash))
	for _, txnHash := range txnsHash {
		if count[txnHash] > 0 {
			return true
		}
		count[txnHash]++
	}
	return false
}
