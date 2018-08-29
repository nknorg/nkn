package chord

import (
	"bytes"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"time"
)

func CompareId(id1, id2 []byte) int {
	l1, l2 := len(id1), len(id2)
	if l1 > l2 {
		return -CompareId(id2, id1)
	}
	if l1 < l2 {
		tmp := make([]byte, l2)
		copy(tmp[l2-l1:], id1)
		id1 = tmp
	}
	return bytes.Compare(id1, id2)
}

func bigIntToId(i big.Int, m int) []byte {
	b := i.Bytes()
	lb := len(b)
	lId := m / 8
	if lb < lId {
		id := make([]byte, lId)
		copy(id[lId-lb:], b)
		return id
	}
	if lb > lId {
		log.Println("[WARNING] Big integer has more bytes than ID.")
	}
	return b
}

// Generates a random stabilization time
func randStabilize(conf *Config) time.Duration {
	min := conf.StabilizeMin
	max := conf.StabilizeMax
	r := rand.Float64()
	return time.Duration((r * float64(max-min)) + float64(min))
}

// Checks if a key is STRICTLY between two ID's exclusively
func between(id1, id2, key []byte) bool {
	// Check for ring wrap around
	if CompareId(id1, id2) == 1 {
		return CompareId(id1, key) == -1 ||
			CompareId(id2, key) == 1
	}

	// Handle the normal case
	return CompareId(id1, key) == -1 &&
		CompareId(id2, key) == 1
}

// Checks if a key is between two ID's, right inclusive
func betweenRightIncl(id1, id2, key []byte) bool {
	// Check for ring wrap around
	if CompareId(id1, id2) == 1 {
		return CompareId(id1, key) == -1 ||
			CompareId(id2, key) >= 0
	}

	return CompareId(id1, key) == -1 &&
		CompareId(id2, key) >= 0
}

// Computes the offset by (n + 2^exp) % (2^mod)
func powerOffset(id []byte, exp int, mod int) []byte {
	// Copy the existing slice
	off := make([]byte, len(id))
	copy(off, id)

	// Convert the ID to a bigint
	idInt := big.Int{}
	idInt.SetBytes(id)

	// Get the offset
	two := big.NewInt(2)
	offset := big.Int{}
	offset.Exp(two, big.NewInt(int64(exp)), nil)

	// Sum
	sum := big.Int{}
	sum.Add(&idInt, &offset)

	// Get the ceiling
	ceil := big.Int{}
	ceil.Exp(two, big.NewInt(int64(mod)), nil)

	// Apply the mod
	idInt.Mod(&sum, &ceil)

	return bigIntToId(idInt, mod)
}

// max returns the max of two ints
func max(a, b int) int {
	if a >= b {
		return a
	} else {
		return b
	}
}

// min returns the min of two ints
func min(a, b int) int {
	if a <= b {
		return a
	} else {
		return b
	}
}

// Returns the vnode nearest a key
func nearestVnodeToKey(vnodes []*Vnode, key []byte) *Vnode {
	for i := len(vnodes) - 1; i >= 0; i-- {
		if CompareId(vnodes[i].Id, key) == -1 {
			return vnodes[i]
		}
	}
	// Return the last vnode
	return vnodes[len(vnodes)-1]
}

// Merges errors together
func mergeErrors(err1, err2 error) error {
	if err1 == nil {
		return err2
	} else if err2 == nil {
		return err1
	} else {
		return fmt.Errorf("%s\n%s", err1, err2)
	}
}
