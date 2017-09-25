package abi

import (
	"reflect"
	"fmt"
)

func formatSliceString(kind reflect.Kind, sliceSize int) string {
	if sliceSize == -1 {
		return fmt.Sprintf("[]%v", kind)
	}
	return fmt.Sprintf("[%d]%v", sliceSize, kind)
}

func sliceTypeCheck(t Type, val reflect.Value) error {
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return typeErr(formatSliceString(t.Kind, t.SliceSize), val.Type())
	}
	if t.IsArray && val.Len() != t.SliceSize {
		return typeErr(formatSliceString(t.Elem.Kind, t.SliceSize), formatSliceString(val.Type().Elem().Kind(), val.Len()))
	}

	if t.Elem.IsSlice {
		if val.Len() > 0 {
			return sliceTypeCheck(*t.Elem, val.Index(0))
		}
	} else if t.Elem.IsArray {
		return sliceTypeCheck(*t.Elem, val.Index(0))
	}

	if elemKind := val.Type().Elem().Kind(); elemKind != t.Elem.Kind {
		return typeErr(formatSliceString(t.Elem.Kind, t.SliceSize), val.Type())
	}
	return nil
}


func typeCheck(t Type, value reflect.Value) error {
	if t.IsSlice || t.IsArray {
		return sliceTypeCheck(t, value)
	}
	fmt.Println("t.kind", t.Kind, value.Kind())
	// Check base type validity. Element types will be checked later on.
	if t.Kind != value.Kind() {
		return typeErr(t.Kind, value.Kind())
	}
	return nil
}

func typeErr(expected, got interface{}) error {
	return fmt.Errorf("abi: cannot use %v as type %v as argument", got, expected)
}
