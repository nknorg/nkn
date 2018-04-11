package ising

import (
	"io"

	"github.com/nknorg/nkn/common/serialization"
)

type StateProbe struct {
	message string
}

func (p *StateProbe) Serialize(w io.Writer) error {
	err := serialization.WriteVarBytes(w, []byte(p.message))
	if err != nil {
		return err
	}

	return nil
}

func (p *StateProbe) Deserialize(r io.Reader) error {
	msg, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	p.message = string(msg)

	return nil
}
