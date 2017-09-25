package abi

import (
	"fmt"
	"encoding/json"
)

type Argument struct {
	Name    string
	Type    Type
	Indexed bool // indexed is only used by events
}

func (a *Argument) UnmarshalJSON(data []byte) error {
	var extarg struct {
		Name    string
		Type    string
		Indexed bool
	}
	err := json.Unmarshal(data, &extarg)
	if err != nil {
		return fmt.Errorf("argument json err: %v", err)
	}

	a.Type, err = NewType(extarg.Type)
	if err != nil {
		return err
	}
	a.Name = extarg.Name
	a.Indexed = extarg.Indexed

	return nil
}