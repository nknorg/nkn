package abi

import (
	"reflect"
	"math/big"
	"DNA/vm/evm/common"
	. "DNA/common"
)

var (
	big_t     = reflect.TypeOf(big.Int{})
	ubig_t    = reflect.TypeOf(big.Int{})
	address_t = reflect.TypeOf(Uint160{})
)


func U256(n *big.Int) []byte {
	return common.PaddedBigBytes(n, 32)
}

func packNum(value reflect.Value) []byte {
	switch kind := value.Kind(); kind {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return U256(new(big.Int).SetUint64(value.Uint()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return U256(big.NewInt(value.Int()))
	case reflect.Ptr:
		return U256(value.Interface().(*big.Int))
	}
	return nil
}
