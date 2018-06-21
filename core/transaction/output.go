package transaction

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/nknorg/nkn/common"
)

type TxnOutput struct {
	AssetID     common.Uint256
	Value       common.Fixed64
	ProgramHash common.Uint160
}

func (output *TxnOutput) Serialize(w io.Writer) error {
	_, err := output.AssetID.Serialize(w)
	if err != nil {
		return err
	}
	err = output.Value.Serialize(w)
	if err != nil {
		return err
	}
	_, err = output.ProgramHash.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (output *TxnOutput) Deserialize(r io.Reader) error {
	err := output.AssetID.Deserialize(r)
	if err != nil {
		return err
	}
	err = output.Value.Deserialize(r)
	if err != nil {
		return err
	}
	err = output.ProgramHash.Deserialize(r)
	if err != nil {
		return err
	}

	return nil
}

func (output *TxnOutput) Equal(o2 *TxnOutput) bool {
	if output.AssetID.CompareTo(o2.AssetID) != 0 {
		return false
	}

	if output.Value.GetData() != o2.Value.GetData() {
		return false
	}

	if output.ProgramHash.CompareTo(o2.ProgramHash) != 0 {
		return false
	}

	return true
}

func (output *TxnOutput) MarshalJson() ([]byte, error) {
	address, _ := output.ProgramHash.ToAddress()
	outputInfo := TxnOutputInfo{
		AssetID: common.BytesToHexString(output.AssetID.ToArrayReverse()),
		Value:   output.Value.String(),
		Address: address,
	}

	data, err := json.Marshal(outputInfo)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (output *TxnOutput) UnmarshalJson(data []byte) error {
	outputInfo := new(TxnOutputInfo)
	var err error
	if err = json.Unmarshal(data, &outputInfo); err != nil {
		return err
	}

	assetID, err := common.HexStringToBytesReverse(outputInfo.AssetID)
	if err != nil {
		return err
	}
	output.AssetID, err = common.Uint256ParseFromBytes(assetID)
	if err != nil {
		return err
	}

	output.Value, err = common.StringToFixed64(outputInfo.Value)
	if err != nil {
		return err
	}

	output.ProgramHash, err = common.ToScriptHash(outputInfo.Address)
	//fmt.Println(common.ToScriptHash(outputInfo.Address))

	if err != nil {
		return err
	}

	return nil
}

func (output *TxnOutput) ToArray() []byte {
	b := new(bytes.Buffer)
	output.Serialize(b)
	return b.Bytes()
}
