package abi

import (
	"reflect"
	"fmt"
)

func indirect(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Ptr && v.Elem().Type() != big_t {
		return indirect(v.Elem())
	}
	return v
}

func mustArrayToByteSlice(value reflect.Value) reflect.Value {
	slice := reflect.MakeSlice(reflect.TypeOf([]byte{}), value.Len(), value.Len())
	reflect.Copy(slice, value)
	return slice
}

func set(dst, src reflect.Value, output Argument) error {
	dstType := dst.Type()
	srcType := src.Type()

	switch {
	case dstType.AssignableTo(src.Type()):
		dst.Set(src)
	case dstType.Kind() == reflect.Array && srcType.Kind() == reflect.Slice:
		if dst.Len() < output.Type.SliceSize {
			return fmt.Errorf("abi: cannot unmarshal src (len=%d) in to dst (len=%d)", output.Type.SliceSize, dst.Len())
		}
		reflect.Copy(dst, src)
	case dstType.Kind() == reflect.Interface:
		dst.Set(src)
	case dstType.Kind() == reflect.Ptr:
		return set(dst.Elem(), src, output)
	default:
		return fmt.Errorf("abi: cannot unmarshal %v in to %v", src.Type(), dst.Type())
	}
	return nil
}

