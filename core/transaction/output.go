package transaction

import (
	"github.com/nknorg/nkn/common"
	"io"
	"bytes"
)

type TxOutput struct {
	AssetID     common.Uint256
	Value       common.Fixed64
	ProgramHash common.Uint160
}

func (o *TxOutput) Serialize(w io.Writer) error {
	_, err := o.AssetID.Serialize(w)
	if err != nil {
		return err
	}
	err = o.Value.Serialize(w)
	if err != nil {
		return err
	}
	_, err = o.ProgramHash.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (o *TxOutput) Deserialize(r io.Reader) error {
	err := o.AssetID.Deserialize(r)
	if err != nil {
		return err
	}
	err = o.Value.Deserialize(r)
	if err != nil {
		return err
	}
	err = o.ProgramHash.Deserialize(r)
	if err != nil {
		return err
	}

	return nil
}

func (ui *TxOutput) ToArray() ([]byte) {
	b := new(bytes.Buffer)
	ui.Serialize(b)
	return b.Bytes()
}
