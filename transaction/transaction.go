package transaction

import (
	"crypto/sha256"
	"encoding/json"
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	. "github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/vm/signature"
)

func NewMsgTx(payload *Payload, nonce uint64, fee Fixed64, attrs []byte) *MsgTx {
	unsigned := &UnsignedTx{
		Payload:    payload,
		Nonce:      nonce,
		Fee:        int64(fee),
		Attributes: attrs,
	}

	tx := &MsgTx{
		UnsignedTx: unsigned,
	}

	return tx
}

type Transaction struct {
	MsgTx
	hash *Uint256
}

func (tx *Transaction) Marshal() (dAtA []byte, err error) {
	return tx.MsgTx.Marshal()
}

func (tx *Transaction) Unmarshal(dAtA []byte) error {
	return tx.MsgTx.Unmarshal(dAtA)
}

//Serialize the Transaction
func (tx *Transaction) Serialize(w io.Writer) error {
	return nil
}

//Serialize the Transaction data without contracts
func (tx *Transaction) SerializeUnsigned(w io.Writer) error {
	if err := tx.UnsignedTx.Payload.Serialize(w); err != nil {
		return err
	}

	err := serialization.WriteUint64(w, uint64(tx.UnsignedTx.Nonce))
	if err != nil {
		return err
	}

	err = serialization.WriteUint64(w, uint64(tx.UnsignedTx.Fee))
	if err != nil {
		return err
	}

	return nil
}

//deserialize the Transaction
//TODO
func (tx *Transaction) Deserialize(r io.Reader) error {
	data, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	err = tx.Unmarshal(data)
	return err
}

func (tx *Transaction) DeserializeUnsigned(r io.Reader) error {
	err := tx.UnsignedTx.Payload.Deserialize(r)
	if err != nil {
		return err
	}

	tx.UnsignedTx.Nonce, err = serialization.ReadUint64(r)
	if err != nil {
		return err
	}

	fee, err := serialization.ReadUint64(r)
	if err != nil {
		return err
	}
	tx.UnsignedTx.Fee = int64(fee)

	return nil
}

func (tx *Transaction) GetProgramHashes() ([]Uint160, error) {
	hashes := []Uint160{}

	switch tx.UnsignedTx.Payload.Type {
	case TransferAssetType:
		payload, err := Unpack(tx.UnsignedTx.Payload)
		if err != nil {
			return nil, err
		}

		sender := payload.(*TransferAsset).Sender
		hashes = append(hashes, BytesToUint160(sender))
	default:
		//hash, _ := ToCodeHash(tx.Program[0].Code)
		//hashes = append(hashes, hash)
	}

	return hashes, nil
}

func (tx *Transaction) SetPrograms(programs []*Program) {
	tx.Programs = programs
}

func (tx *Transaction) GetPrograms() []*Program {
	return tx.Programs
}

func (tx *Transaction) GetMessage() []byte {
	return signature.GetHashData(tx)
}

func (tx *Transaction) ToArray() []byte {
	dt, _ := tx.Marshal()
	return dt
}

func (tx *Transaction) Hash() Uint256 {
	if tx.hash == nil {
		d := signature.GetHashData(tx)
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
