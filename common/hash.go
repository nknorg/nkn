package common

import "math/big"

const (
	HashLength    = 32
)

type Hash [HashLength]byte

func BytesToHash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}

func (h *Hash) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-HashLength:]
	}
	copy(h[HashLength-len(b):], b)
}

func BigToHash(b *big.Int) Hash  { return BytesToHash(b.Bytes()) }
func (h Hash) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }
func (h Hash) Bytes() []byte { return h[:] }
