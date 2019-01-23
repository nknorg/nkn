package transaction

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/contract/program"
	sig "github.com/nknorg/nkn/core/signature"
	"github.com/nknorg/nkn/core/transaction/payload"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/util/log"
)

//for different transaction types with different payload format
//and transaction process methods
type TransactionType byte

const (
	Coinbase      TransactionType = 0x00
	TransferAsset TransactionType = 0x10
	Commit        TransactionType = 0x42
	RegisterName  TransactionType = 0x50
	TransferName  TransactionType = 0x51
	DeleteName    TransactionType = 0x52
	Subscribe     TransactionType = 0x60
)

type TransactionResult map[Uint256]Fixed64

//Payload define the func for loading the payload data
//base on payload type which have different struture
type Payload interface {
	//  Get payload data
	Data(version byte) []byte

	//Serialize payload data
	Serialize(w io.Writer, version byte) error

	Deserialize(r io.Reader, version byte) error

	MarshalJson() ([]byte, error)

	UnmarshalJson(data []byte) error
}

//Transaction is used for carry information or action to Ledger
//validated transaction will be added to block and updates state correspondingly

var Store TxnStore

type Transaction struct {
	TxType         TransactionType
	PayloadVersion byte
	Payload        Payload
	Fee            Fixed64
	Attributes     []*TxnAttribute
	Programs       []*program.Program

	hash *Uint256
}

//Serialize the Transaction
func (tx *Transaction) Serialize(w io.Writer) error {

	err := tx.SerializeUnsigned(w)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Transaction txSerializeUnsigned Serialize failed.")
	}
	//Serialize  Transaction's programs
	lens := uint64(len(tx.Programs))
	err = serialization.WriteVarUint(w, lens)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Transaction WriteVarUint failed.")
	}
	if lens > 0 {
		for _, p := range tx.Programs {
			err = p.Serialize(w)
			if err != nil {
				return NewDetailErr(err, ErrNoCode, "Transaction Programs Serialize failed.")
			}
		}
	}
	return nil
}

//Serialize the Transaction data without contracts
func (tx *Transaction) SerializeUnsigned(w io.Writer) error {
	//txType
	w.Write([]byte{byte(tx.TxType)})
	//PayloadVersion
	w.Write([]byte{tx.PayloadVersion})
	//Payload
	if tx.Payload == nil {
		return errors.New("Transaction Payload is nil.")
	}
	tx.Payload.Serialize(w, tx.PayloadVersion)
	tx.Fee.Serialize(w)
	//[]*txAttribute
	err := serialization.WriteVarUint(w, uint64(len(tx.Attributes)))
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Transaction item txAttribute length serialization failed.")
	}
	if len(tx.Attributes) > 0 {
		for _, attr := range tx.Attributes {
			attr.Serialize(w)
		}
	}

	return nil
}

//deserialize the Transaction
func (tx *Transaction) Deserialize(r io.Reader) error {
	// tx deserialize
	err := tx.DeserializeUnsigned(r)
	if err != nil {
		log.Error("Deserialize DeserializeUnsigned:", err)
		return NewDetailErr(err, ErrNoCode, "transaction Deserialize error")
	}

	// tx program
	lens, err := serialization.ReadVarUint(r, 0)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "transaction tx program Deserialize error")
	}

	programHashes := []*program.Program{}
	if lens > 0 {
		for i := 0; i < int(lens); i++ {
			outputHashes := new(program.Program)
			outputHashes.Deserialize(r)
			programHashes = append(programHashes, outputHashes)
		}
		tx.Programs = programHashes
	}
	return nil
}

func (tx *Transaction) DeserializeUnsigned(r io.Reader) error {
	var txType [1]byte
	_, err := io.ReadFull(r, txType[:])
	if err != nil {
		log.Error("DeserializeUnsigned ReadFull:", err)
		return err
	}
	tx.TxType = TransactionType(txType[0])
	return tx.DeserializeUnsignedWithoutType(r)
}

func (tx *Transaction) DeserializeUnsignedWithoutType(r io.Reader) error {
	var payloadVersion [1]byte
	_, err := io.ReadFull(r, payloadVersion[:])
	tx.PayloadVersion = payloadVersion[0]
	if err != nil {
		log.Error("DeserializeUnsignedWithoutType:", err)
		return err
	}

	//payload
	//tx.Payload.Deserialize(r)
	switch tx.TxType {
	case TransferAsset:
		tx.Payload = new(payload.TransferAsset)
	case Coinbase:
		tx.Payload = new(payload.Coinbase)
	case Commit:
		tx.Payload = new(payload.Commit)
	case RegisterName:
		tx.Payload = new(payload.RegisterName)
	case DeleteName:
		tx.Payload = new(payload.DeleteName)
	case Subscribe:
		tx.Payload = new(payload.Subscribe)
	default:
		return errors.New("[Transaction],invalid transaction type.")
	}
	err = tx.Payload.Deserialize(r, tx.PayloadVersion)
	if err != nil {
		log.Error("tx Payload Deserialize:", err)
		return NewDetailErr(err, ErrNoCode, "Payload Parse error")
	}

	err = tx.Fee.Deserialize(r)
	//attributes
	Len, err := serialization.ReadVarUint(r, 0)
	if err != nil {
		log.Error("tx attributes Deserialize:", err)
		return err
	}
	if Len > uint64(0) {
		for i := uint64(0); i < Len; i++ {
			attr := new(TxnAttribute)
			err = attr.Deserialize(r)
			if err != nil {
				return err
			}
			tx.Attributes = append(tx.Attributes, attr)
		}
	}

	return nil
}

func (tx *Transaction) GetProgramHashes() ([]Uint160, error) {
	hashes := []Uint160{}

	switch tx.TxType {
	case TransferAsset:
		sender := tx.Payload.(*payload.TransferAsset).Sender
		hashes = append(hashes, sender)
	default:
	}

	return hashes, nil
}

func (tx *Transaction) SetPrograms(programs []*program.Program) {
	tx.Programs = programs
}

func (tx *Transaction) GetPrograms() []*program.Program {
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
	var txInfo TransactionInfo

	txInfo.TxType = tx.TxType
	txInfo.PayloadVersion = tx.PayloadVersion

	if tx.Payload != nil {
		info, err := tx.Payload.MarshalJson()
		if err != nil {
			return nil, err
		}
		var t interface{}
		json.Unmarshal(info, &t)

		txInfo.Payload = t
	}

	txInfo.Fee = tx.Fee.String()

	for _, v := range tx.Attributes {
		info, err := v.MarshalJson()
		if err != nil {
			return nil, err
		}
		var t TxnAttributeInfo
		json.Unmarshal(info, &t)
		txInfo.Attributes = append(txInfo.Attributes, t)
	}

	for _, v := range tx.Programs {
		info, err := v.MarshalJson()
		if err != nil {
			return nil, err
		}
		var t program.ProgramInfo
		json.Unmarshal(info, &t)
		txInfo.Programs = append(txInfo.Programs, t)
	}

	hash := tx.Hash()
	txInfo.Hash = BytesToHexString(hash.ToArrayReverse())

	data, err := json.Marshal(txInfo)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (tx *Transaction) UnmarshalJson(data []byte) error {
	txInfo := new(TransactionInfo)
	var err error
	if err = json.Unmarshal(data, &txInfo); err != nil {
		return err
	}

	tx.TxType = txInfo.TxType
	tx.PayloadVersion = txInfo.PayloadVersion

	info, err := json.Marshal(txInfo.Payload)
	if err != nil {
		return err
	}
	switch tx.TxType {
	case TransferAsset:
		tx.Payload = new(payload.TransferAsset)
	case Coinbase:
		tx.Payload = new(payload.Coinbase)
	case Commit:
		tx.Payload = new(payload.Commit)
	case RegisterName:
		tx.Payload = new(payload.RegisterName)
	case DeleteName:
		tx.Payload = new(payload.DeleteName)
	case Subscribe:
		tx.Payload = new(payload.Subscribe)
	default:
		return errors.New("[Transaction],invalid transaction type.")
	}
	err = tx.Payload.UnmarshalJson(info)
	if err != nil {
		return err
	}

	tx.Fee, _ = StringToFixed64(txInfo.Fee)

	for _, v := range txInfo.Attributes {
		info, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var at TxnAttribute
		err = at.UnmarshalJson(info)
		if err != nil {
			return err
		}
		tx.Attributes = append(tx.Attributes, &at)
	}

	for _, v := range txInfo.Programs {
		info, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var p program.Program
		err = p.UnmarshalJson(info)
		if err != nil {
			return err
		}
		tx.Programs = append(tx.Programs, &p)
	}

	if txInfo.Hash != "" {
		hashSlice, err := HexStringToBytesReverse(txInfo.Hash)
		if err != nil {
			return err
		}
		hash, err := Uint256ParseFromBytes(hashSlice)
		if err != nil {
			return err
		}
		tx.hash = &hash
	}

	return nil
}
