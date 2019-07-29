package pb

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/nknorg/nkn/common/serialization"
)

//Serialize the Program
func (p *Program) Serialize(w io.Writer) error {
	err := serialization.WriteVarBytes(w, p.Parameter)
	if err != nil {
		return fmt.Errorf("Execute Program Serialize Code failed:%v", err)
	}
	err = serialization.WriteVarBytes(w, p.Code)
	if err != nil {
		return fmt.Errorf("Execute Program Serialize Parameter failed:%v", err)
	}

	return nil
}

//Deserialize the Program
func (p *Program) Deserialize(w io.Reader) error {
	val, err := serialization.ReadVarBytes(w)
	if err != nil {
		return fmt.Errorf("Execute Program Deserialize Parameter failed:%v", err)
	}
	p.Parameter = val
	p.Code, err = serialization.ReadVarBytes(w)
	if err != nil {
		return fmt.Errorf("Execute Program Deserialize Code failed:%v", err)
	}
	return nil
}

func (p *Program) MarshalJson() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Program) UnmarshalJson(data []byte) error {
	return json.Unmarshal(data, p)
}

func (p *Payload) Serialize(w io.Writer) error {
	err := serialization.WriteUint32(w, uint32(p.Type))
	if err != nil {
		return err
	}
	err = serialization.WriteVarBytes(w, p.Data)
	if err != nil {
		return err
	}

	return nil
}

func (p *Payload) Deserialize(r io.Reader) error {
	typ, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	p.Type = PayloadType(typ)
	dat, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	p.Data = dat

	return nil
}
