package states

import (
	"io"
	"DNA/common/serialization"
	. "DNA/errors"
	"bytes"
)

type StorageItem struct {
	Value []byte
	*StateBase
}

func NewStorageItem(value []byte) *StorageItem {
	var storageItem StorageItem
	storageItem.Value = value
	return &storageItem
}

func(storageItem *StorageItem)Serialize(w io.Writer) error {
	serialization.WriteVarBytes(w, storageItem.Value)
	return nil
}

func(storageItem *StorageItem)Deserialize(r io.Reader) error {
	value, err := serialization.ReadVarBytes(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "ContractState Code Deserialize fail.")
	}
	storageItem.Value = value
	return nil
}

func(storageItem *StorageItem) ToArray() []byte {
	b := new(bytes.Buffer)
	storageItem.Serialize(b)
	return b.Bytes()
}