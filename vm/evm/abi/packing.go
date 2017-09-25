package abi

import (
	"reflect"
	"DNA/vm/evm/common"
)

func packBytesSlice(bytes []byte, l int) []byte {
	len := packNum(reflect.ValueOf(l))
	return append(len, common.RightPadBytes(bytes, (l+31)/32*32)...)
}

func packElement(t Type, reflectValue reflect.Value) []byte {
	switch t.T {
	case IntTy, UintTy:
		return packNum(reflectValue)
	case StringTy:
		return packBytesSlice([]byte(reflectValue.String()), reflectValue.Len())
	case AddressTy:
		if reflectValue.Kind() == reflect.Array {
			reflectValue = mustArrayToByteSlice(reflectValue)
		}

		return common.LeftPadBytes(reflectValue.Bytes(), 32)
	case BoolTy:
		if reflectValue.Bool() {
			return common.PaddedBigBytes(common.Big1, 32)
		} else {
			return common.PaddedBigBytes(common.Big0, 32)
		}
	case BytesTy:
		if reflectValue.Kind() == reflect.Array {
			reflectValue = mustArrayToByteSlice(reflectValue)
		}
		return packBytesSlice(reflectValue.Bytes(), reflectValue.Len())
	case FixedBytesTy, FunctionTy:
		if reflectValue.Kind() == reflect.Array {
			reflectValue = mustArrayToByteSlice(reflectValue)
		}
		return common.RightPadBytes(reflectValue.Bytes(), 32)
	}
	return []byte{}
}

func reflectIntKind(unsigned bool, size int) reflect.Kind {
	switch size {
	case 8:
		if unsigned {
			return reflect.Uint8
		}
		return reflect.Int8
	case 16:
		if unsigned {
			return reflect.Uint16
		}
		return reflect.Int16
	case 32:
		if unsigned {
			return reflect.Uint32
		}
		return reflect.Int32
	case 64:
		if unsigned {
			return reflect.Uint64
		}
		return reflect.Int64
	}
	return reflect.Ptr
}

