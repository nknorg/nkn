package evm

import (
	"math/big"
	"DNA/common"
)


type destinations map[common.Uint160][]byte

func (d destinations) has(codeHash common.Uint160, code []byte, dest *big.Int) bool {
	udest := dest.Uint64()

	if dest.BitLen() >= 63 || udest >= uint64(len(code)) {
		return false
	}

	m, analysed := d[codeHash]
	if !analysed {
		m = jumpdest(code)
		d[codeHash] = m
	}
	return (m[udest/8] & (1 << (udest % 8))) != 0
}

func jumpdest(code []byte) []byte {
	m :=  make([]byte, len(code)/8+1)
	for pc := uint64(0); pc < uint64(len(code)); pc++ {
		var op OpCode = OpCode(code[pc])
		if op == JUMPDEST {
			m[pc/8] |= 1 << (pc % 8)
		} else if op >= PUSH1 && op <= PUSH32 {
			a := uint64(op) - uint64(PUSH1) + 1
			pc += a
		}
	}
	return m
}
