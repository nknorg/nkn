package transaction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type TxnInput struct {

	//Indicate the previous Tx which include the UTXO output for usage
	ReferTxID common.Uint256

	//The index of output in the referTx output list
	ReferTxOutputIndex uint16
}

func (input *TxnInput) Serialize(w io.Writer) error {
	_, err := input.ReferTxID.Serialize(w)
	if err != nil {
		return err
	}
	err = serialization.WriteUint16(w, input.ReferTxOutputIndex)
	if err != nil {
		return err
	}

	return nil
}

func (input *TxnInput) Deserialize(r io.Reader) error {
	//referTxID
	err := input.ReferTxID.Deserialize(r)
	if err != nil {
		return err
	}

	//Output Index
	temp, err := serialization.ReadUint16(r)
	input.ReferTxOutputIndex = uint16(temp)
	if err != nil {
		return err
	}

	return nil
}

func (input *TxnInput) Equal(ui2 *TxnInput) bool {
	if input.ReferTxID.CompareTo(ui2.ReferTxID) != 0 {
		return false
	}

	if input.ReferTxOutputIndex != input.ReferTxOutputIndex {
		return false
	}

	return true
}

func (input *TxnInput) MarshalJson() ([]byte, error) {

	inputInfo := TxnInputInfo{
		ReferTxID:          common.BytesToHexString(input.ReferTxID.ToArrayReverse()),
		ReferTxOutputIndex: input.ReferTxOutputIndex,
	}

	data, err := json.Marshal(inputInfo)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (input *TxnInput) UnmarshalJson(data []byte) error {

	inputInfo := new(TxnInputInfo)
	var err error
	if err = json.Unmarshal(data, &inputInfo); err != nil {
		return err
	}

	txid, err := common.HexStringToBytesReverse(inputInfo.ReferTxID)
	if err != nil {
		return err
	}
	input.ReferTxID, err = common.Uint256ParseFromBytes(txid)
	if err != nil {
		return err
	}
	input.ReferTxOutputIndex = inputInfo.ReferTxOutputIndex

	return nil
}

func (input *TxnInput) ToArray() []byte {
	b := new(bytes.Buffer)
	input.Serialize(b)
	return b.Bytes()
}

func (input *TxnInput) ToString() string {
	return fmt.Sprintf("%x%x", input.ReferTxID.ToString(), input.ReferTxOutputIndex)
}

func (input *TxnInput) Equals(other *TxnInput) bool {
	if input == other {
		return true
	}
	if other == nil {
		return false
	}
	if input.ReferTxID == other.ReferTxID && input.ReferTxOutputIndex == other.ReferTxOutputIndex {
		return true
	} else {
		return false
	}
}
