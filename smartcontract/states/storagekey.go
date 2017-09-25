package states

import (
	"DNA/common"
	"io"
	"DNA/common/serialization"
	. "DNA/errors"
)

type StorageKey struct {
	CodeHash *common.Uint160
	Key []byte
}

func NewStorageKey(codeHash *common.Uint160, key []byte) *StorageKey {
	var storageKey StorageKey
	storageKey.CodeHash = codeHash
	storageKey.Key = key
	return &storageKey
}

func (storageKey *StorageKey) Serialize(w io.Writer) (int, error) {
	storageKey.CodeHash.Serialize(w)
	serialization.WriteVarBytes(w, storageKey.Key)
	return 0, nil
}

func (storageKey *StorageKey) Deserialize(r io.Reader) error {
	u := new(common.Uint160)
	err := u.Deserialize(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "StorageKey CodeHash Deserialize fail.")
	}
	storageKey.CodeHash = u
	key, err := serialization.ReadVarBytes(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "StorageKey Key Deserialize fail.")
	}
	storageKey.Key = key
	return nil
}

