package evm

import (
	"testing"
	"math/big"
)

func TestOpAdd(t *testing.T) {
	evm := NewExecutionEngine(nil, nil, nil)
	evm.stack.push(big.NewInt(2))
	evm.stack.push(big.NewInt(3))
	opAdd(evm)
	t.Log("add execute resutl:", evm.stack.pop())
}

func TestOpMul(t *testing.T) {
	evm := NewExecutionEngine(nil, nil, nil)
	evm.stack.push(big.NewInt(2))
	evm.stack.push(big.NewInt(3))
	opMul(evm)
	t.Log("Mul execute resutl:", evm.stack.pop())
}

func TestOpSub(t *testing.T) {
	evm := NewExecutionEngine(nil, nil, nil)
	evm.stack.push(big.NewInt(2))
	evm.stack.push(big.NewInt(3))
	opSub(evm)
	t.Log("Mul execute resutl:", evm.stack.pop())
}

func TestOpDiv(t *testing.T) {
	evm := NewExecutionEngine(nil, nil, nil)
	evm.stack.push(big.NewInt(-2))
	evm.stack.push(big.NewInt(3))
	opDiv(evm)
	t.Log("Mul execute resutl:", evm.stack.pop())
}

func TestOpSdiv(t *testing.T) {
	evm := NewExecutionEngine(nil, nil, nil)
	evm.stack.push(big.NewInt(-2))
	evm.stack.push(big.NewInt(3))
	opSdiv(evm)
	t.Log("Mul execute resutl:", evm.stack.pop())
}