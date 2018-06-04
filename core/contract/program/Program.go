package program

import (
	"encoding/json"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	. "github.com/nknorg/nkn/errors"
)

type Program struct {

	//the contract program code,which will be run on VM or specific envrionment
	Code []byte

	//the program code's parameter
	Parameter []byte
}

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

func (p *Program) Equal(b *Program) bool {
	if !common.IsEqualBytes(p.Code, b.Code) {
		return false
	}

	if !common.IsEqualBytes(p.Parameter, b.Parameter) {
		return false
	}

	return true
}

func (p *Program) MarshalJson() ([]byte, error) {
	program := ProgramInfo{
		Code:      common.BytesToHexString(p.Code),
		Parameter: common.BytesToHexString(p.Parameter),
	}

	data, err := json.Marshal(program)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (p *Program) UnmarshalJson(data []byte) error {
	program := new(ProgramInfo)
	var err error
	if err = json.Unmarshal(data, &program); err != nil {
		return err
	}

	p.Code, err = common.HexStringToBytes(program.Code)
	if err != nil {
		return nil
	}

	p.Parameter, err = common.HexStringToBytes(program.Parameter)
	if err != nil {
		return nil
	}

	return nil
}
