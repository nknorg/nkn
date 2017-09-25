package abi

import (
	"fmt"
	"io"
	"encoding/json"
	"reflect"
	"strings"
	"encoding/binary"
	"math/big"
	"DNA/common"
)

var (
	r_interSlice = reflect.TypeOf([]interface{}{})
)

type ABI struct {
	Constructor Method
	Methods     map[string]Method
	Events      map[string]Event
}

func JSON(reader io.Reader) (ABI, error) {
	dec := json.NewDecoder(reader)

	var abi ABI
	if err := dec.Decode(&abi); err != nil {
		return ABI{}, err
	}

	return abi, nil
}

func (abi *ABI) Pack(name string, args ...interface{}) ([]byte, error) {
	var method Method
	if name == "" {
		method = abi.Constructor
	} else {
		m, exist := abi.Methods[name]
		if !exist {
			return nil, fmt.Errorf("method '%s' not found", name)
		}
		method = m
	}

	arguments, err := method.pack(method, args...)

	if err != nil {
		return nil, err
	}

	if name == "" {
		return arguments, nil
	}

	return append(method.Id(), arguments...), nil
}

func (abi *ABI) UnmarshalJSON(data []byte) error {
	var fields []struct {
		Type      string
		Name      string
		Constant  bool
		Indexed   bool
		Anonymous bool
		Inputs    []Argument
		Outputs   []Argument
	}

	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	abi.Methods = make(map[string]Method)
	abi.Events = make(map[string]Event)
	for _, field := range fields {
		switch field.Type {
		case "constructor":
			abi.Constructor = Method{
				Inputs: field.Inputs,
			}
		// empty defaults to function according to the abi spec
		case "function", "":
			abi.Methods[field.Name] = Method{
				Name:    field.Name,
				Const:   field.Constant,
				Inputs:  field.Inputs,
				Outputs: field.Outputs,
			}
		case "event":
			abi.Events[field.Name] = Event{
				Name:      field.Name,
				Anonymous: field.Anonymous,
				Inputs:    field.Inputs,
			}
		}
	}

	return nil
}

func (abi ABI) Unpack(v interface{}, name string, output []byte) error {
	var method = abi.Methods[name]

	if len(output) == 0 {
		return fmt.Errorf("abi: unmarshalling empty output")
	}

	// make sure the passed value is a pointer
	valueOf := reflect.ValueOf(v)
	if reflect.Ptr != valueOf.Kind() {
		return fmt.Errorf("abi: Unpack(non-pointer %T)", v)
	}

	var (
		value = valueOf.Elem()
		typ   = value.Type()
	)

	if len(method.Outputs) > 1 {
		switch value.Kind() {
		case reflect.Struct:
			for i := 0; i < len(method.Outputs); i++ {
				marshalledValue, err := toGoType(i, method.Outputs[i], output)
				if err != nil {
					return err
				}
				reflectValue := reflect.ValueOf(marshalledValue)

				for j := 0; j < typ.NumField(); j++ {
					field := typ.Field(j)
					if field.Name == strings.ToUpper(method.Outputs[i].Name[:1])+method.Outputs[i].Name[1:] {
						if err := set(value.Field(j), reflectValue, method.Outputs[i]); err != nil {
							return err
						}
					}
				}
			}
		case reflect.Slice:
			if !value.Type().AssignableTo(r_interSlice) {
				return fmt.Errorf("abi: cannot marshal tuple in to slice %T (only []interface{} is supported)", v)
			}

			if value.Len() > 0 {
				if len(method.Outputs) > value.Len() {
					return fmt.Errorf("abi: cannot marshal in to slices of unequal size (require: %v, got: %v)", len(method.Outputs), value.Len())
				}

				for i := 0; i < len(method.Outputs); i++ {
					marshalledValue, err := toGoType(i, method.Outputs[i], output)
					if err != nil {
						return err
					}
					reflectValue := reflect.ValueOf(marshalledValue)
					if err := set(value.Index(i).Elem(), reflectValue, method.Outputs[i]); err != nil {
						return err
					}
				}
				return nil
			}

			z := reflect.MakeSlice(typ, 0, len(method.Outputs))
			for i := 0; i < len(method.Outputs); i++ {
				marshalledValue, err := toGoType(i, method.Outputs[i], output)
				if err != nil {
					return err
				}
				z = reflect.Append(z, reflect.ValueOf(marshalledValue))
			}
			value.Set(z)
		default:
			return fmt.Errorf("abi: cannot unmarshal tuple in to %v", typ)
		}

	} else {
		marshalledValue, err := toGoType(0, method.Outputs[0], output)
		if err != nil {
			return err
		}
		if err := set(value, reflect.ValueOf(marshalledValue), method.Outputs[0]); err != nil {
			return err
		}
	}

	return nil
}

func toGoType(i int, t Argument, output []byte) (interface{}, error) {
	// we need to treat slices differently
	if (t.Type.IsSlice || t.Type.IsArray) && t.Type.T != BytesTy && t.Type.T != StringTy && t.Type.T != FixedBytesTy && t.Type.T != FunctionTy {
		return toGoSlice(i, t, output)
	}

	index := i * 32
	if index+32 > len(output) {
		return nil, fmt.Errorf("abi: cannot marshal in to go type: length insufficient %d require %d", len(output), index+32)
	}

	var returnOutput []byte
	switch t.Type.T {
	case StringTy, BytesTy:
		offset := int(binary.BigEndian.Uint64(output[index+24 : index+32]))
		if offset+32 > len(output) {
			return nil, fmt.Errorf("abi: cannot marshal in to go type: length insufficient %d require %d", len(output), offset+32)
		}
		size := int(binary.BigEndian.Uint64(output[offset+24 : offset+32]))
		if offset+32+size > len(output) {
			return nil, fmt.Errorf("abi: cannot marshal in to go type: length insufficient %d require %d", len(output), offset+32+size)
		}

		returnOutput = output[offset+32 : offset+32+size]
	default:
		returnOutput = output[index : index+32]
	}

	// convert the bytes to whatever is specified by the ABI.
	switch t.Type.T {
	case IntTy, UintTy:
		return readInteger(t.Type.Kind, returnOutput), nil
	case BoolTy:
		return !allZero(returnOutput), nil
	case AddressTy:
		return common.BytesToUint160(returnOutput), nil
	case HashTy:
		return common.BytesToHash(returnOutput), nil
	case BytesTy, FixedBytesTy, FunctionTy:
		return returnOutput, nil
	case StringTy:
		return string(returnOutput), nil
	}
	return nil, fmt.Errorf("abi: unknown type %v", t.Type.T)
}

func allZero(b []byte) bool {
	for _, byte := range b {
		if byte != 0 {
			return false
		}
	}
	return true
}

func readInteger(kind reflect.Kind, b []byte) interface{} {
	switch kind {
	case reflect.Uint8:
		return uint8(b[len(b)-1])
	case reflect.Uint16:
		return binary.BigEndian.Uint16(b[len(b)-2:])
	case reflect.Uint32:
		return binary.BigEndian.Uint32(b[len(b)-4:])
	case reflect.Uint64:
		return binary.BigEndian.Uint64(b[len(b)-8:])
	case reflect.Int8:
		return int8(b[len(b)-1])
	case reflect.Int16:
		return int16(binary.BigEndian.Uint16(b[len(b)-2:]))
	case reflect.Int32:
		return int32(binary.BigEndian.Uint32(b[len(b)-4:]))
	case reflect.Int64:
		return int64(binary.BigEndian.Uint64(b[len(b)-8:]))
	default:
		return new(big.Int).SetBytes(b)
	}
}

func toGoSlice(i int, t Argument, output []byte) (interface{}, error) {
	index := i * 32
	if index+32 > len(output) {
		return nil, fmt.Errorf("abi: cannot marshal in to go slice: insufficient size output %d require %d", len(output), index+32)
	}
	elem := t.Type.Elem

	var refSlice reflect.Value
	switch elem.T {
	case IntTy, UintTy, BoolTy:
		switch t.Type.Kind {
		case reflect.Bool:
			refSlice = reflect.ValueOf([]bool(nil))
		case reflect.Uint8:
			refSlice = reflect.ValueOf([]uint8(nil))
		case reflect.Uint16:
			refSlice = reflect.ValueOf([]uint16(nil))
		case reflect.Uint32:
			refSlice = reflect.ValueOf([]uint32(nil))
		case reflect.Uint64:
			refSlice = reflect.ValueOf([]uint64(nil))
		case reflect.Int8:
			refSlice = reflect.ValueOf([]int8(nil))
		case reflect.Int16:
			refSlice = reflect.ValueOf([]int16(nil))
		case reflect.Int32:
			refSlice = reflect.ValueOf([]int32(nil))
		case reflect.Int64:
			refSlice = reflect.ValueOf([]int64(nil))
		default:
			refSlice = reflect.ValueOf([]*big.Int(nil))
		}
	case AddressTy:
		refSlice = reflect.ValueOf([]common.Uint160(nil))
	case HashTy:
		refSlice = reflect.ValueOf([]common.Hash(nil))
	case FixedBytesTy:
		refSlice = reflect.ValueOf([][]byte(nil))
	default:
		return nil, fmt.Errorf("abi: unsupported slice type %v", elem.T)
	}

	var slice []byte
	var size int
	var offset int
	if t.Type.IsSlice {
		offset = int(binary.BigEndian.Uint64(output[index+24 : index+32]))
		if offset+32 > len(output) {
			return nil, fmt.Errorf("abi: cannot marshal in to go slice: offset %d would go over slice boundary (len=%d)", len(output), offset+32)
		}

		slice = output[offset:]
		size = int(binary.BigEndian.Uint64(slice[24:32]))
		slice = slice[32:]
		if size*32 > len(slice) {
			return nil, fmt.Errorf("abi: cannot marshal in to go slice: insufficient size output %d require %d", len(output), offset+32+size*32)
		}

		slice = slice[:size*32]
	} else if t.Type.IsArray {
		size = t.Type.SliceSize

		if index+32*size > len(output) {
			return nil, fmt.Errorf("abi: cannot marshal in to go array: offset %d would go over slice boundary (len=%d)", len(output), index+32*size)
		}
		slice = output[index : index+size*32]
	}

	for i := 0; i < size; i++ {
		var (
			inter        interface{}
			returnOutput = slice[i*32 : i*32+32]
		)
		switch elem.T {
		case IntTy, UintTy:
			inter = readInteger(t.Type.Kind, returnOutput)
		case BoolTy:
			inter = !allZero(returnOutput)
		case AddressTy:
			inter = common.BytesToUint160(returnOutput)
		case HashTy:
			inter = common.BytesToHash(returnOutput)
		case FixedBytesTy:
			inter = returnOutput
		}
		refSlice = reflect.Append(refSlice, reflect.ValueOf(inter))
	}

	return refSlice.Interface(), nil
}

