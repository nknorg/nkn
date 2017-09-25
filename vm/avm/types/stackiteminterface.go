package types

import (
	"math/big"
	"DNA/vm/avm/interfaces"
)

type StackItemInterface interface {
	Equals(other StackItemInterface) bool
	GetBigInteger() *big.Int
	GetBoolean() bool
	GetByteArray() []byte
	GetInterface() interfaces.IInteropInterface
	GetArray() []StackItemInterface
}

