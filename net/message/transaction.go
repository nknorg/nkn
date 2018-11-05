package message

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	. "github.com/nknorg/nkn/errors"
	. "github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/log"
)

type dataReq struct {
	msgHdr
	dataType common.InventoryType
	hash     common.Uint256
}

// Transaction message
type trn struct {
	msgHdr
	txn transaction.Transaction
}

func (msg trn) Handle(node Noder) error {
	txn := &msg.txn
	if !node.LocalNode().ExistHash(txn.Hash()) {
		node.LocalNode().IncRxTxnCnt()

		// add transaction to pool when in consensus state
		if node.LocalNode().GetSyncState() == PersistFinished {
			if errCode := node.LocalNode().AppendTxnPool(txn); errCode != ErrNoError {
				return fmt.Errorf("[message] VerifyTransaction failed with %v when AppendTxnPool.", errCode)
			}
		}
	}

	return nil
}

func (msg dataReq) Serialize(w io.Writer) error {
	err := msg.msgHdr.Serialize(w)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, msg.dataType)
	if err != nil {
		return err
	}
	_, err = msg.hash.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (msg *dataReq) Deserialize(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &(msg.msgHdr))
	if err != nil {
		log.Error("datareq header message parsing error")
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &(msg.dataType))
	if err != nil {
		log.Error("datareq datatype message parsing error")
		return err
	}
	err = msg.hash.Deserialize(r)
	if err != nil {
		log.Error("datareq hash message parsing error")
		return err
	}

	return nil
}

func NewTxnFromHash(hash common.Uint256) (*transaction.Transaction, error) {
	txn, err := ledger.DefaultLedger.GetTransactionWithHash(hash)
	if err != nil {
		log.Error("Get transaction with hash error: ", err.Error())
		return nil, err
	}

	return txn, nil
}
func NewTxn(txn *transaction.Transaction) ([]byte, error) {
	var msg trn
	msg.msgHdr.Magic = NetID
	cmd := "tx"
	copy(msg.msgHdr.CMD[0:len(cmd)], cmd)
	tmpBuffer := bytes.NewBuffer([]byte{})
	txn.Serialize(tmpBuffer)
	msg.txn = *txn
	b := new(bytes.Buffer)
	err := binary.Write(b, binary.LittleEndian, tmpBuffer.Bytes())
	if err != nil {
		log.Error("Binary Write failed at new Msg")
		return nil, err
	}
	s := sha256.Sum256(b.Bytes())
	s2 := s[:]
	s = sha256.Sum256(s2)
	buf := bytes.NewBuffer(s[:4])
	binary.Read(buf, binary.LittleEndian, &(msg.msgHdr.Checksum))
	msg.msgHdr.Length = uint32(len(b.Bytes()))

	txnBuff := bytes.NewBuffer(nil)
	err = msg.Serialize(txnBuff)
	if err != nil {
		log.Error("Error Convert net message ", err.Error())
		return nil, err
	}

	return txnBuff.Bytes(), nil
}

func (msg trn) Serialize(w io.Writer) error {
	err := msg.msgHdr.Serialize(w)
	if err != nil {
		return err
	}
	err = msg.txn.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (msg *trn) Deserialize(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &(msg.msgHdr))
	if err != nil {
		return err
	}
	err = msg.txn.Deserialize(r)
	if err != nil {
		return err
	}

	return nil
}
