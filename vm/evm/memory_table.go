package evm

import (
	"math/big"
	"DNA/vm/evm/common"
)

func memoryMStore(stack *Stack) *big.Int {
	return common.CalcMemSize(stack.Back(0), big.NewInt(32))
}