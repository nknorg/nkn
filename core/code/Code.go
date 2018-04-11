package code

import (
	. "github.com/nknorg/nkn/common"
	. "github.com/nknorg/nkn/core/contract"
)
//ICode is the abstract interface of smart contract code.
type ICode interface {

	GetCode() []byte

	GetParameterTypes() []ContractParameterType

	GetReturnTypes() []ContractParameterType

	CodeHash() Uint160

}

