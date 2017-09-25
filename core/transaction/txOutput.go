package transaction

import (
	"DNA/common"
	"io"
	"bytes"
)

type TxOutput struct {
	AssetID     common.Uint256
	Value       common.Fixed64
	ProgramHash common.Uint160
}

func (o *TxOutput) Serialize(w io.Writer) {
	o.AssetID.Serialize(w)
	o.Value.Serialize(w)
	o.ProgramHash.Serialize(w)
}

func (o *TxOutput) Deserialize(r io.Reader) {
	o.AssetID.Deserialize(r)
	o.Value.Deserialize(r)
	o.ProgramHash.Deserialize(r)
}

func (ui *TxOutput) ToArray() ([]byte) {
	b := new(bytes.Buffer)
	ui.Serialize(b)
	return b.Bytes()
}