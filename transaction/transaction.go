package transaction

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"

	"github.com/gogo/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/vm/contract"
	"github.com/nknorg/nkn/vm/signature"
)

type Transaction struct {
	*pb.Transaction
	hash                *Uint256
	isSignatureVerified bool
}

func (tx *Transaction) Marshal() (buf []byte, err error) {
	return proto.Marshal(tx.Transaction)
}

func (tx *Transaction) Unmarshal(buf []byte) error {
	if tx.Transaction == nil {
		tx.Transaction = &pb.Transaction{}
	}
	return proto.Unmarshal(buf, tx.Transaction)
}

func NewMsgTx(payload *pb.Payload, nonce uint64, fee Fixed64, attrs []byte) *pb.Transaction {
	unsigned := &pb.UnsignedTx{
		Payload:    payload,
		Nonce:      nonce,
		Fee:        int64(fee),
		Attributes: attrs,
	}

	tx := &pb.Transaction{
		UnsignedTx: unsigned,
	}

	return tx
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

	err = serialization.WriteVarBytes(w, tx.UnsignedTx.Attributes)
	if err != nil {
		return err
	}

	return nil
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

	val, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	tx.UnsignedTx.Attributes = val

	return nil
}

func (tx *Transaction) GetSize() int {
	marshaledTx, _ := tx.Marshal()
	return len(marshaledTx)
}

func (tx *Transaction) GetProgramHashes() ([]Uint160, error) {
	hashes := []Uint160{}

	payload, err := Unpack(tx.UnsignedTx.Payload)
	if err != nil {
		return nil, err
	}

	switch tx.UnsignedTx.Payload.Type {
	case pb.CommitType:
		sender := payload.(*pb.Commit).Submitter
		hashes = append(hashes, BytesToUint160(sender))
	case pb.TransferAssetType:
		sender := payload.(*pb.TransferAsset).Sender
		hashes = append(hashes, BytesToUint160(sender))
	case pb.CoinbaseType:
		sender := payload.(*pb.Coinbase).Sender
		hashes = append(hashes, BytesToUint160(sender))
	case pb.RegisterNameType:
		pubkey := payload.(*pb.RegisterName).Registrant
		publicKey, err := crypto.NewPubKeyFromBytes(pubkey)
		if err != nil {
			return nil, err
		}
		programhash, err := contract.CreateRedeemHash(publicKey)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, programhash)
	case pb.DeleteNameType:
		pubkey := payload.(*pb.DeleteName).Registrant
		publicKey, err := crypto.NewPubKeyFromBytes(pubkey)
		if err != nil {
			return nil, err
		}
		programhash, err := contract.CreateRedeemHash(publicKey)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, programhash)
	case pb.SubscribeType:
		pubkey := payload.(*pb.Subscribe).Subscriber
		publicKey, err := crypto.NewPubKeyFromBytes(pubkey)
		if err != nil {
			return nil, err
		}
		programhash, err := contract.CreateRedeemHash(publicKey)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, programhash)
	case pb.GenerateIDType:
		pubkey := payload.(*pb.GenerateID).PublicKey
		publicKey, err := crypto.NewPubKeyFromBytes(pubkey)
		if err != nil {
			return nil, err
		}
		programhash, err := contract.CreateRedeemHash(publicKey)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, programhash)
	case pb.NanoPayType:
		sender := payload.(*pb.NanoPay).Sender
		hashes = append(hashes, BytesToUint160(sender))
	default:
		return nil, errors.New("unsupport transaction type")
	}

	return hashes, nil
}

func (tx *Transaction) SetPrograms(programs []*pb.Program) {
	tx.Programs = programs
}

func (tx *Transaction) GetPrograms() []*pb.Program {
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

func HashToShortHash(hash Uint256, salt []byte, size uint32) []byte {
	shortHash := sha256.Sum256(append(hash[:], salt...))
	if size > sha256.Size {
		return shortHash[:]
	}
	return shortHash[:size]
}

func (tx *Transaction) ShortHash(salt []byte, size uint32) []byte {
	return HashToShortHash(tx.Hash(), salt, size)
}

func (tx *Transaction) SetHash(hash Uint256) {
	tx.hash = &hash
}

func (txn *Transaction) VerifySignature() error {
	if txn.UnsignedTx.Payload.Type == pb.CoinbaseType {
		return nil
	}

	if txn.isSignatureVerified {
		return nil
	}

	if err := signature.VerifySignableData(txn); err != nil {
		return err
	}

	txn.isSignatureVerified = true

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

func (tx *Transaction) GetInfo() ([]byte, error) {
	type programInfo struct {
		Code      string `json:"code"`
		Parameter string `json:"parameter"`
	}
	type txnInfo struct {
		TxType      string        `json:"txType"`
		PayloadData string        `json:"payloadData"`
		Nonce       uint64        `json:"nonce"`
		Fee         int64         `json:"fee"`
		Attributes  string        `json:"attributes"`
		Programs    []programInfo `json:"programs"`
		Hash        string        `json:"hash"`
	}

	tx.Hash()
	info := &txnInfo{
		TxType:      tx.UnsignedTx.Payload.GetType().String(),
		PayloadData: BytesToHexString(tx.UnsignedTx.Payload.GetData()),
		Nonce:       tx.UnsignedTx.Nonce,
		Fee:         tx.UnsignedTx.Fee,
		Attributes:  BytesToHexString(tx.UnsignedTx.Attributes),
		Programs:    make([]programInfo, 0),
		Hash:        tx.hash.ToHexString(),
	}

	for _, v := range tx.Programs {
		pgInfo := &programInfo{}
		pgInfo.Code = BytesToHexString(v.Code)
		pgInfo.Parameter = BytesToHexString(v.Parameter)
		info.Programs = append(info.Programs, *pgInfo)
	}

	marshaledInfo, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}
	return marshaledInfo, nil

}
