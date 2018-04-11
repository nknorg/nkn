package types

import (
	"math/big"
	"github.com/nknorg/nkn/vm/interfaces"
)

type StackItemInterface interface {
	Equals(other StackItemInterface) bool
	GetBigInteger() *big.Int
	GetBoolean() bool
	GetByteArray() []byte
	GetInterface() interfaces.IInteropInterface
	GetArray() []StackItemInterface
}

