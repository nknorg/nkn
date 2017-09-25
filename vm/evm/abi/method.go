package abi

import (
	"fmt"
	"strings"
	"DNA/vm/evm/crypto"
	"reflect"
)

type Method struct {
	Name    string
	Const   bool
	Inputs  []Argument
	Outputs []Argument
}

func (m Method) pack(method Method, args ...interface{}) ([]byte, error) {
	if len(args) != len(method.Inputs) {
		return nil, fmt.Errorf("argument count mismatch: %d for %d", len(args), len(method.Inputs))
	}
	var variableInput []byte

	var ret []byte

	for i, a := range args {
		input := method.Inputs[i]
		packed, err := input.Type.pack(reflect.ValueOf(a))
		if err != nil {
			return nil, fmt.Errorf("`%s` %v", method.Name, err)
		}
		// check for a slice type (string, bytes, slice)
		if input.Type.requiresLengthPrefix() {
			// calculate the offset
			offset := len(method.Inputs)*32 + len(variableInput)
			// set the offset
			ret = append(ret, packNum(reflect.ValueOf(offset))...)
			// Append the packed output to the variable input. The variable input
			// will be appended at the end of the input.
			variableInput = append(variableInput, packed...)
		} else {
			// append the packed value to the input
			ret = append(ret, packed...)
		}
	}
	// append the variable input at the end of the packed input
	ret = append(ret, variableInput...)

	return ret, nil
}

func (m Method) Sig() string {
	types := make([]string, len(m.Inputs))
	i := 0
	for _, input := range m.Inputs {
		types[i] = input.Type.String()
		i++
	}
	return fmt.Sprintf("%v(%v)", m.Name, strings.Join(types, ","))
}

func (m Method) Id() []byte {
	return crypto.Keccak256([]byte(m.Sig()))[:4]
}
