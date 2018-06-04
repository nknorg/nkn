package program

import (
	"fmt"
	"testing"
)

func TestProgram(t *testing.T) {
	p := &Program{
		Code:      []byte{1, 2, 3, 4},
		Parameter: []byte{5, 6, 7, 8},
	}

	data, err := p.MarshalJson()
	if err != nil {
		t.Error("Program MarshalJson error")
	}

	program := new(Program)
	err = program.UnmarshalJson(data)
	if err != nil {
		t.Error("Program unmarshalJson error")
	}

	if !program.Equal(p) {
		fmt.Println(p)
		fmt.Println(program)
		t.Error("Program Compare error")
	}
}
