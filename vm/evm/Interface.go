package evm

import (
	"DNA/common"
	"math/big"
)

type StateDB interface {
	GetState(common.Uint160, common.Hash) common.Hash
	SetState(common.Uint160, common.Hash, common.Hash)

	GetCode(common.Uint160) []byte
	SetCode(common.Uint160, []byte)
	GetCodeSize(common.Uint160) int

	GetBalance(common.Uint160) *big.Int
	AddBalance(common.Uint160, *big.Int)

	Suicide(common.Uint160) bool
}
