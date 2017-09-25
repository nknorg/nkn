package abi

import (
	"reflect"
	"fmt"
	"strconv"
	"regexp"
)

type Type struct {
	IsSlice, IsArray bool
	SliceSize        int

	Elem *Type

	Kind reflect.Kind
	Type reflect.Type
	Size int
	T    byte // Our own type checking

	stringKind string // holds the unparsed string for deriving signatures
}

const (
	IntTy byte = iota
	UintTy
	BoolTy
	StringTy
	SliceTy
	AddressTy
	FixedBytesTy
	BytesTy
	HashTy
	FixedpointTy
	FunctionTy
)

var (
	fullTypeRegex = regexp.MustCompile(`([a-zA-Z0-9]+)(\[([0-9]*)\])?`)
	typeRegex = regexp.MustCompile("([a-zA-Z]+)(([0-9]+)(x([0-9]+))?)?")
)

func NewType(t string) (typ Type, err error) {
	res := fullTypeRegex.FindAllStringSubmatch(t, -1)[0]
	// check if type is slice and parse type.
	switch {
	case res[3] != "":
		// err is ignored. Already checked for number through the regexp
		typ.SliceSize, _ = strconv.Atoi(res[3])
		typ.IsArray = true
	case res[2] != "":
		typ.IsSlice, typ.SliceSize = true, -1
	case res[0] == "":
		return Type{}, fmt.Errorf("abi: type parse error: %s", t)
	}
	if typ.IsArray || typ.IsSlice {
		sliceType, err := NewType(res[1])
		if err != nil {
			return Type{}, err
		}
		typ.Elem = &sliceType
		typ.stringKind = sliceType.stringKind + t[len(res[1]):]
		// Although we know that this is an array, we cannot return
		// as we don't know the type of the element, however, if it
		// is still an array, then don't determine the type.
		if typ.Elem.IsArray || typ.Elem.IsSlice {
			return typ, nil
		}
	}

	// parse the type and size of the abi-type.
	parsedType := typeRegex.FindAllStringSubmatch(res[1], -1)[0]
	// varSize is the size of the variable
	var varSize int
	if len(parsedType[3]) > 0 {
		var err error
		varSize, err = strconv.Atoi(parsedType[2])
		if err != nil {
			return Type{}, fmt.Errorf("abi: error parsing variable size: %v", err)
		}
	}
	// varType is the parsed abi type
	varType := parsedType[1]
	// substitute canonical integer
	if varSize == 0 && (varType == "int" || varType == "uint") {
		varSize = 256
		t += "256"
	}

	// only set stringKind if not array or slice, as for those,
	// the correct string type has been set
	if !(typ.IsArray || typ.IsSlice) {
		typ.stringKind = t
	}

	switch varType {
	case "int":
		typ.Kind = reflectIntKind(false, varSize)
		typ.Type = big_t
		typ.Size = varSize
		typ.T = IntTy
	case "uint":
		typ.Kind = reflectIntKind(true, varSize)
		typ.Type = ubig_t
		typ.Size = varSize
		typ.T = UintTy
	case "bool":
		typ.Kind = reflect.Bool
		typ.T = BoolTy
	case "address":
		typ.Kind = reflect.Array
		typ.Type = address_t
		typ.Size = 20
		typ.T = AddressTy
	case "string":
		typ.Kind = reflect.String
		typ.Size = -1
		typ.T = StringTy
	case "bytes":
		sliceType, _ := NewType("uint8")
		typ.Elem = &sliceType
		if varSize == 0 {
			typ.IsSlice = true
			typ.T = BytesTy
			typ.SliceSize = -1
		} else {
			typ.IsArray = true
			typ.T = FixedBytesTy
			typ.SliceSize = varSize
		}
	case "function":
		sliceType, _ := NewType("uint8")
		typ.Elem = &sliceType
		typ.IsArray = true
		typ.T = FunctionTy
		typ.SliceSize = 24
	default:
		return Type{}, fmt.Errorf("unsupported arg type: %s", t)
	}

	return
}


func (t Type) String() (out string) {
	return t.stringKind
}

func (t Type) pack(v reflect.Value) ([]byte, error) {
	v = indirect(v)
	if err := typeCheck(t, v); err != nil {
		return nil, err
	}

	if (t.IsSlice || t.IsArray) && t.T != BytesTy && t.T != FixedBytesTy && t.T != FunctionTy {
		var packed []byte

		for i := 0; i < v.Len(); i++ {
			val, err := t.Elem.pack(v.Index(i))
			if err != nil {
				return nil, err
			}
			packed = append(packed, val...)
		}
		if t.IsSlice {
			return packBytesSlice(packed, v.Len()), nil
		} else if t.IsArray {
			return packed, nil
		}
	}

	return packElement(t, v), nil
}

func (t Type) requiresLengthPrefix() bool {
	return t.T != FixedBytesTy && (t.T == StringTy || t.T == BytesTy || t.IsSlice)
}
