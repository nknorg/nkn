package transaction

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	. "github.com/nknorg/nkn/errors"
)

type TransactionAttributeUsage byte

const (
	Nonce       TransactionAttributeUsage = 0x00
	Script      TransactionAttributeUsage = 0x20
	Description TransactionAttributeUsage = 0x90
)

func IsValidAttributeType(usage TransactionAttributeUsage) bool {
	return usage == Nonce || usage == Script || usage == Description
}

type TxnAttribute struct {
	Usage TransactionAttributeUsage
	Data  []byte
	Size  uint32
}

func NewTxAttribute(u TransactionAttributeUsage, d []byte) TxnAttribute {
	tx := TxnAttribute{u, d, 0}
	tx.Size = tx.GetSize()
	return tx
}

func (attr *TxnAttribute) GetSize() uint32 {
	return 0
}

func (attr *TxnAttribute) Serialize(w io.Writer) error {
	if err := serialization.WriteUint8(w, byte(attr.Usage)); err != nil {
		return NewDetailErr(err, ErrNoCode, "Transaction attribute Usage serialization error.")
	}
	if !IsValidAttributeType(attr.Usage) {
		return NewDetailErr(errors.New("[TxnAttribute] error"), ErrNoCode, "Unsupported attribute Description.")
	}
	if err := serialization.WriteVarBytes(w, attr.Data); err != nil {
		return NewDetailErr(err, ErrNoCode, "Transaction attribute Data serialization error.")
	}
	return nil
}

func (attr *TxnAttribute) Deserialize(r io.Reader) error {
	val, err := serialization.ReadBytes(r, 1)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Transaction attribute Usage deserialization error.")
	}
	attr.Usage = TransactionAttributeUsage(val[0])
	if !IsValidAttributeType(attr.Usage) {
		return NewDetailErr(errors.New("[TxnAttribute] error"), ErrNoCode, "Unsupported attribute Description.")
	}
	attr.Data, err = serialization.ReadVarBytes(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Transaction attribute Data deserialization error.")
	}
	return nil

}

func (attr *TxnAttribute) Equal(tx2 *TxnAttribute) bool {
	if attr.Usage != tx2.Usage {
		return false
	}

	if !common.IsEqualBytes(attr.Data, tx2.Data) {
		return false
	}

	return true
}

func (attr *TxnAttribute) MarshalJson() ([]byte, error) {
	txnInfo := TxnAttributeInfo{
		Usage: attr.Usage,
		Data:  common.BytesToHexString(attr.Data),
	}

	data, err := json.Marshal(txnInfo)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (attr *TxnAttribute) UnmarshalJson(data []byte) error {
	txnInfo := new(TxnAttributeInfo)
	var err error
	if err = json.Unmarshal(data, &txnInfo); err != nil {
		return err
	}

	attr.Usage = txnInfo.Usage
	attr.Data, err = common.HexStringToBytes(txnInfo.Data)
	if err != nil {
		return nil
	}

	attr.Size = attr.GetSize()

	return nil
}

func (attr *TxnAttribute) ToArray() []byte {
	b := new(bytes.Buffer)
	attr.Serialize(b)
	return b.Bytes()
}
