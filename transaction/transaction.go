package transaction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/common/serialization"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/program"
	"github.com/nknorg/nkn/v2/signature"
)

type Transaction struct {
	*pb.Transaction
	hash                *common.Uint256
	size                uint32
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

func NewMsgTx(payload *pb.Payload, nonce uint64, fee common.Fixed64, attrs []byte) *pb.Transaction {
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

func (tx *Transaction) GetSize() uint32 {
	if tx.size == 0 {
		marshaledTx, _ := tx.Marshal()
		tx.size = uint32(len(marshaledTx))
	}
	return tx.size
}

func (tx *Transaction) GetProgramHashes() ([]common.Uint160, error) {
	hashes := []common.Uint160{}

	payload, err := Unpack(tx.UnsignedTx.Payload)
	if err != nil {
		return nil, err
	}

	switch tx.UnsignedTx.Payload.Type {
	case pb.PayloadType_SIG_CHAIN_TXN_TYPE:
		sender := payload.(*pb.SigChainTxn).Submitter
		hashes = append(hashes, common.BytesToUint160(sender))
	case pb.PayloadType_TRANSFER_ASSET_TYPE:
		sender := payload.(*pb.TransferAsset).Sender
		hashes = append(hashes, common.BytesToUint160(sender))
	case pb.PayloadType_COINBASE_TYPE:
		sender := payload.(*pb.Coinbase).Sender
		hashes = append(hashes, common.BytesToUint160(sender))
	case pb.PayloadType_REGISTER_NAME_TYPE:
		publicKey := payload.(*pb.RegisterName).Registrant
		programhash, err := program.CreateProgramHash(publicKey)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, programhash)
	case pb.PayloadType_TRANSFER_NAME_TYPE:
		publicKey := payload.(*pb.TransferName).Registrant
		programhash, err := program.CreateProgramHash(publicKey)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, programhash)
	case pb.PayloadType_DELETE_NAME_TYPE:
		publicKey := payload.(*pb.DeleteName).Registrant
		programhash, err := program.CreateProgramHash(publicKey)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, programhash)
	case pb.PayloadType_SUBSCRIBE_TYPE:
		publicKey := payload.(*pb.Subscribe).Subscriber
		programhash, err := program.CreateProgramHash(publicKey)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, programhash)
	case pb.PayloadType_UNSUBSCRIBE_TYPE:
		publicKey := payload.(*pb.Unsubscribe).Subscriber
		programhash, err := program.CreateProgramHash(publicKey)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, programhash)
	case pb.PayloadType_GENERATE_ID_TYPE:
		genID := payload.(*pb.GenerateID)
		var programhash common.Uint160
		if len(genID.Sender) > 0 {
			programhash = common.BytesToUint160(genID.Sender)
		} else {
			programhash, err = program.CreateProgramHash(genID.PublicKey)
			if err != nil {
				return nil, err
			}
		}
		hashes = append(hashes, programhash)
	case pb.PayloadType_NANO_PAY_TYPE:
		sender := payload.(*pb.NanoPay).Sender
		hashes = append(hashes, common.BytesToUint160(sender))
	case pb.PayloadType_ISSUE_ASSET_TYPE:
		sender := payload.(*pb.IssueAsset).Sender
		hashes = append(hashes, common.BytesToUint160(sender))
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

func (tx *Transaction) Hash() common.Uint256 {
	if tx.hash == nil {
		d := signature.GetHashData(tx)
		temp := sha256.Sum256([]byte(d))
		f := common.Uint256(sha256.Sum256(temp[:]))
		tx.hash = &f
	}
	return *tx.hash
}

func HashToShortHash(hash common.Uint256, salt []byte, size uint32) []byte {
	shortHash := sha256.Sum256(append(hash[:], salt...))
	if size > sha256.Size {
		return shortHash[:]
	}
	return shortHash[:size]
}

func (tx *Transaction) ShortHash(salt []byte, size uint32) []byte {
	return HashToShortHash(tx.Hash(), salt, size)
}

func (tx *Transaction) SetHash(hash common.Uint256) {
	tx.hash = &hash
}

func (txn *Transaction) VerifySignature() error {
	if txn.UnsignedTx.Payload.Type == pb.PayloadType_COINBASE_TYPE {
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

type byProgramHashes []common.Uint160

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
		Size        uint32        `json:"size"`
	}

	tx.Hash()
	info := &txnInfo{
		TxType:      tx.UnsignedTx.Payload.GetType().String(),
		PayloadData: hex.EncodeToString(tx.UnsignedTx.Payload.GetData()),
		Nonce:       tx.UnsignedTx.Nonce,
		Fee:         tx.UnsignedTx.Fee,
		Attributes:  hex.EncodeToString(tx.UnsignedTx.Attributes),
		Programs:    make([]programInfo, 0),
		Hash:        tx.hash.ToHexString(),
		Size:        tx.GetSize(),
	}

	for _, v := range tx.Programs {
		pgInfo := &programInfo{}
		pgInfo.Code = hex.EncodeToString(v.Code)
		pgInfo.Parameter = hex.EncodeToString(v.Parameter)
		info.Programs = append(info.Programs, *pgInfo)
	}

	marshaledInfo, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}
	return marshaledInfo, nil

}
