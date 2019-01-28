package transaction

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	sig "github.com/nknorg/nkn/signature"
	"github.com/nknorg/nkn/types"
)

type TransactionResult map[Uint256]Fixed64

var Store TxnStore

type Transaction struct {
	types.MsgTx
	hash *Uint256
}

//Serialize the Transaction
func (tx *Transaction) Serialize(w io.Writer) error {
	data, err := tx.Marshal()
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, data)
	return err
}

//Serialize the Transaction data without contracts
func (tx *Transaction) SerializeUnsigned(w io.Writer) error {
	data, err := tx.UnsignedTx.Marshal()
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, data)
	return err
}

//deserialize the Transaction
func (tx *Transaction) Deserialize(r io.Reader) error {
	data, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	err = tx.Unmarshal(data)
	return err
}

func (tx *Transaction) DeserializeUnsigned(r io.Reader) error {
	data, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	err = tx.UnsignedTx.Unmarshal(data)
	return err
}

func (tx *Transaction) GetProgramHashes() ([]Uint160, error) {
	hashes := []Uint160{}

	switch tx.UnsignedTx.Payload.Type {
	case types.TransferAssetType:
		payload, err := types.Unpack(tx.UnsignedTx.Payload)
		if err != nil {
			return nil, err
		}

		sender := payload.(*types.TransferAsset).Sender
		hashes = append(hashes, BytesToUint160(sender))
	default:
	}

	return hashes, nil
}

func (tx *Transaction) SetPrograms(programs []*types.Program) {
	tx.Programs = programs
}

func (tx *Transaction) GetPrograms() []*types.Program {
	return tx.Programs
}

func (tx *Transaction) GetMessage() []byte {
	return sig.GetHashData(tx)
}

func (tx *Transaction) ToArray() []byte {
	b := new(bytes.Buffer)
	tx.Serialize(b)
	return b.Bytes()
}

func (tx *Transaction) Hash() Uint256 {
	if tx.hash == nil {
		d := sig.GetHashData(tx)
		temp := sha256.Sum256([]byte(d))
		f := Uint256(sha256.Sum256(temp[:]))
		tx.hash = &f
	}
	return *tx.hash

}

func (tx *Transaction) SetHash(hash Uint256) {
	tx.hash = &hash
}

func (tx *Transaction) Type() InventoryType {
	return TRANSACTION
}
func (tx *Transaction) Verify() error {
	//TODO: Verify()
	return nil
}

type byProgramHashes []Uint160

func (a byProgramHashes) Len() int      { return len(a) }
func (a byProgramHashes) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byProgramHashes) Less(i, j int) bool {
	if a[i].CompareTo(a[j]) > 0 {
		return false
	} else {
		return true
	}
}

func (tx *Transaction) MarshalJson() ([]byte, error) {
	return json.Marshal(tx)
}

func (tx *Transaction) UnmarshalJson(data []byte) error {
	return json.Unmarshal(data, tx)
}
