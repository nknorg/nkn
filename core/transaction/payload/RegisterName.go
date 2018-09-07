package payload

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	. "github.com/nknorg/nkn/errors"
)

type RegisterName struct {
	Registrant []byte
	Name       string
}

func (a *RegisterName) Data(version byte) []byte {
	//TODO: implement RegisterName.Data()
	return []byte{0}

}

func (a *RegisterName) Serialize(w io.Writer, version byte) error {
	serialization.WriteVarBytes(w, a.Registrant)
	serialization.WriteVarString(w, a.Name)
	return nil
}

func (a *RegisterName) Deserialize(r io.Reader, version byte) error {
	var err error
	a.Registrant, err = serialization.ReadVarBytes(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "[RegisterName], Registrant Deserialize failed.")
	}
	a.Name, err = serialization.ReadVarString(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "[RegisterName], Name Deserialize failed.")
	}
	return nil
}

func (a *RegisterName) Equal(b *RegisterName) bool {
	if !bytes.Equal(a.Registrant, b.Registrant) {
		return false
	}

	if a.Name != b.Name {
		return false
	}

	return true
}

func (a *RegisterName) MarshalJson() ([]byte, error) {
	ra := &RegisterNameInfo{
		Registrant: common.BytesToHexString(a.Registrant),
		Name:       a.Name,
	}

	data, err := json.Marshal(ra)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (a *RegisterName) UnmarshalJson(data []byte) error {
	ra := new(RegisterNameInfo)
	var err error
	if err = json.Unmarshal(data, &ra); err != nil {
		return err
	}

	a.Registrant, err = common.HexStringToBytes(ra.Registrant)
	if err != nil {
		return err
	}
	a.Name = ra.Name

	return nil
}
