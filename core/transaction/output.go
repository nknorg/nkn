package transaction

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/nknorg/nkn/common"
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

func (o *TxOutput) Equal(o2 *TxOutput) bool {
	if o.AssetID.CompareTo(o2.AssetID) != 0 {
		return false
	}

	if o.Value.GetData() != o2.Value.GetData() {
		return false
	}

	if o.ProgramHash.CompareTo(o2.ProgramHash) != 0 {
		return false
	}

	return true
}

func (o *TxOutput) MarshalJson() ([]byte, error) {
	address, _ := o.ProgramHash.ToAddress()
	outputInfo := TxOutputInfo{
		AssetID: common.BytesToHexString(o.AssetID.ToArrayReverse()),
		Value:   o.Value.String(),
		Address: address,
	}

	data, err := json.Marshal(outputInfo)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (o *TxOutput) UnmarshalJson(data []byte) error {
	outputInfo := new(TxOutputInfo)
	var err error
	if err = json.Unmarshal(data, &outputInfo); err != nil {
		return err
	}

	assetID, err := common.HexStringToBytesReverse(outputInfo.AssetID)
	if err != nil {
		return err
	}
	o.AssetID, err = common.Uint256ParseFromBytes(assetID)
	if err != nil {
		return err
	}

	o.Value, err = common.StringToFixed64(outputInfo.Value)
	if err != nil {
		return err
	}

	o.ProgramHash, err = common.ToScriptHash(outputInfo.Address)
	//fmt.Println(common.ToScriptHash(outputInfo.Address))

	if err != nil {
		return err
	}

	return nil
}

func (o *TxOutput) ToArray() []byte {
	b := new(bytes.Buffer)
	o.Serialize(b)
	return b.Bytes()
}
