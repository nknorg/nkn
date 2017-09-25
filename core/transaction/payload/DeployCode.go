package payload

import (
	. "DNA/core/code"
	"DNA/common/serialization"
	"io"
	"DNA/smartcontract/types"
	"DNA/common"
)

type DeployCode struct {
	Code        *FunctionCode
	Params      []byte
	Name        string
	CodeVersion string
	Author      string
	Email       string
	Description string
	Language    types.LangType
	ProgramHash common.Uint160
}

func (dc *DeployCode) Data(version byte) []byte {
	// TODO: Data()

	return []byte{0}
}

func (dc *DeployCode) Serialize(w io.Writer, version byte) error {

	err := dc.Code.Serialize(w)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, dc.Params)

	if err != nil {
		return err
	}

	err = serialization.WriteVarString(w, dc.Name)
	if err != nil {
		return err
	}

	err = serialization.WriteVarString(w, dc.CodeVersion)
	if err != nil {
		return err
	}

	err = serialization.WriteVarString(w, dc.Author)
	if err != nil {
		return err
	}

	err = serialization.WriteVarString(w, dc.Email)
	if err != nil {
		return err
	}

	err = serialization.WriteVarString(w, dc.Description)
	if err != nil {
		return err
	}

	err = serialization.WriteByte(w, byte(dc.Language))
	if err != nil {
		return err
	}

	_, err = dc.ProgramHash.Serialize(w)
	if err != nil {
		return err
	}
	return nil
}

func (dc *DeployCode) Deserialize(r io.Reader, version byte) error {
	dc.Code = new(FunctionCode)
	err := dc.Code.Deserialize(r)
	if err != nil {
		return err
	}

	dc.Params, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	dc.Name, err = serialization.ReadVarString(r)
	if err != nil {
		return err
	}

	dc.CodeVersion, err = serialization.ReadVarString(r)
	if err != nil {
		return err
	}

	dc.Author, err = serialization.ReadVarString(r)
	if err != nil {
		return err
	}

	dc.Email, err = serialization.ReadVarString(r)
	if err != nil {
		return err
	}

	dc.Description, err = serialization.ReadVarString(r)
	if err != nil {
		return err
	}

	l, err := serialization.ReadByte(r)
	if err != nil {
		return err
	}
	dc.Language = types.LangType(l)
	u := new(common.Uint160)
	err = u.Deserialize(r)
	if err != nil {
		return err
	}
	dc.ProgramHash = *u
	return nil
}

