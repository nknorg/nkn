package evm

import (
	"DNA/common"
	"math/big"
)


type Contract struct {
	Caller common.Uint160
	Code []byte
	CodeHash common.Uint160
	Input []byte
	value *big.Int
	jumpdest destinations
}

func NewContract(caller common.Uint160) *Contract {
	var contract Contract
	contract.value = new(big.Int)
	contract.jumpdest = make(destinations, 0)
	contract.Caller = caller
	return &contract
}

func (c *Contract) GetOp(n uint64) OpCode {
	return OpCode(c.GetByte(n))
}

func (c *Contract) GetByte(n uint64) byte {
	if n < uint64(len(c.Code)) {
		return c.Code[n]
	}
	return 0
}

func (c *Contract) SetCode(code []byte, codeHash common.Uint160) {
	c.Code = code
	c.CodeHash = codeHash
}

func (c *Contract) SetCallCode(code, input []byte, codeHash common.Uint160) {
	c.Code = code
	c.Input = input
	c.CodeHash = codeHash
}
