package types

import (
	"encoding/json"
	"io"

	"github.com/nknorg/nkn/common/serialization"
	. "github.com/nknorg/nkn/errors"
)

//Serialize the Program
func (p *Program) Serialize(w io.Writer) error {
	err := serialization.WriteVarBytes(w, p.Parameter)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Execute Program Serialize Code failed.")
	}
	err = serialization.WriteVarBytes(w, p.Code)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Execute Program Serialize Parameter failed.")
	}

	return nil
}

//Deserialize the Program
func (p *Program) Deserialize(w io.Reader) error {
	val, err := serialization.ReadVarBytes(w)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Execute Program Deserialize Parameter failed.")
	}
	p.Parameter = val
	p.Code, err = serialization.ReadVarBytes(w)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Execute Program Deserialize Code failed.")
	}
	return nil
}

func (p *Program) MarshalJson() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Program) UnmarshalJson(data []byte) error {
	return json.Unmarshal(data, p)
}
