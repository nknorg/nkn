package payload

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	. "github.com/nknorg/nkn/errors"
)

type DeleteName struct {
	Registrant []byte
}

func (a *DeleteName) Data(version byte) []byte {
	//TODO: implement DeleteName.Data()
	return []byte{0}

}

func (a *DeleteName) Serialize(w io.Writer, version byte) error {
	serialization.WriteVarBytes(w, a.Registrant)
	return nil
}

func (a *DeleteName) Deserialize(r io.Reader, version byte) error {
	var err error
	a.Registrant, err = serialization.ReadVarBytes(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "[RegisterName], Registrant Deserialize failed.")
	}
	return nil
}

func (a *DeleteName) Equal(b *DeleteName) bool {
	if !bytes.Equal(a.Registrant, b.Registrant) {
		return false
	}

	return true
}

func (a *DeleteName) MarshalJson() ([]byte, error) {
	ra := &DeleteNameInfo{
		Registrant: common.BytesToHexString(a.Registrant),
	}

	data, err := json.Marshal(ra)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (a *DeleteName) UnmarshalJson(data []byte) error {
	ra := new(DeleteNameInfo)
	var err error
	if err = json.Unmarshal(data, &ra); err != nil {
		return err
	}

	a.Registrant, err = common.HexStringToBytes(ra.Registrant)
	if err != nil {
		return err
	}

	return nil
}
