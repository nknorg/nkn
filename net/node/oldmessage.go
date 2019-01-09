package node

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	nknErrors "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/util/log"
)

const (
	MsgHdrLen      = 24
	MsgCmdLen      = 12
	MsgCmdOffset   = 4
	MsgChecksumLen = 4
	NetID          = 0x00000000
	MaxHdrCnt      = 500
	MaxInvHdrCnt   = 500
	HashLen        = 32
)

type Messenger interface {
	Verify([]byte) error
	Handle(*RemoteNode) error
	serialization.SerializableData
}

// The network communication message header
type msgHdr struct {
	Magic uint32
	//ID	 uint64
	CMD      [MsgCmdLen]byte // The message type
	Length   uint32
	Checksum [MsgChecksumLen]byte
}

// Alloc different message stucture
// @t the message name or type
// @len the message length only valid for varible length structure
//
// Return:
// @Messenger the Messenger interface
// @error  error code
// FixMe fix the ugly multiple return.
func AllocMsg(t string, length int) Messenger {
	switch t {
	case "msgheader":
		var msg msgHdr
		return &msg
	case "tx":
		var msg trn
		copy(msg.msgHdr.CMD[0:len(t)], t)
		return &msg
	default:
		log.Warning("Unknown message type")
		return nil
	}
}

func MsgType(buf []byte) (string, error) {
	cmd := buf[MsgCmdOffset : MsgCmdOffset+MsgCmdLen]
	n := bytes.IndexByte(cmd, 0)
	if n < 0 || n >= MsgCmdLen {
		return "", errors.New("Unexpected length of CMD command")
	}
	s := string(cmd[:n])
	return s, nil
}

func ParseMsg(buf []byte) (Messenger, error) {
	if len(buf) < MsgHdrLen {
		log.Warning("Unexpected size of received message")
		return nil, errors.New("Unexpected size of received message")
	}

	s, err := MsgType(buf)
	if err != nil {
		log.Error("Message type parsing error")
		return nil, err
	}

	msg := AllocMsg(s, len(buf))
	if msg == nil {
		log.Error(fmt.Sprintf("Allocation message %s failed", s))
		return nil, errors.New("Allocation message failed")
	}
	// Todo attach a node pointer to each message
	// Todo drop the message when verify/deseria packet error
	r := bytes.NewReader(buf[:])
	err = msg.Deserialize(r)
	if err != nil {
		log.Error("Deserialize error:", err)
		return nil, err
	}
	err = msg.Verify(buf[MsgHdrLen:])
	if err != nil {
		log.Error("Verify msg error:", err)
		return nil, err
	}

	return msg, nil
}

func HandleNodeMsg(node *RemoteNode, buf []byte) error {
	msg, err := ParseMsg(buf)
	if err != nil {
		return err
	}

	return msg.Handle(node)
}

func magicVerify(magic uint32) bool {
	return magic == NetID
}

func ValidMsgHdr(buf []byte) bool {
	var h msgHdr
	r := bytes.NewReader(buf)
	h.Deserialize(r)
	//TODO: verify hdr checksum
	return magicVerify(h.Magic)
}

func PayloadLen(buf []byte) int {
	var h msgHdr
	r := bytes.NewReader(buf)
	h.Deserialize(r)
	return int(h.Length)
}

func checkSum(p []byte) []byte {
	t := sha256.Sum256(p)
	s := sha256.Sum256(t[:])

	// Currently we only need the front 4 bytes as checksum
	return s[:MsgChecksumLen]
}

func (hdr *msgHdr) init(cmd string, checksum []byte, length uint32) {
	hdr.Magic = NetID
	copy(hdr.CMD[0:uint32(len(cmd))], cmd)
	copy(hdr.Checksum[:], checksum[:MsgChecksumLen])
	hdr.Length = length
	//hdr.ID = id
}

// Verify the message header information
// @p payload of the message
func (hdr msgHdr) Verify(buf []byte) error {
	if !magicVerify(hdr.Magic) {
		log.Warning(fmt.Sprintf("Unmatched magic number 0x%0x", hdr.Magic))
		return errors.New("Unmatched magic number")
	}
	checkSum := checkSum(buf)
	if !bytes.Equal(hdr.Checksum[:], checkSum[:]) {
		str1 := hex.EncodeToString(hdr.Checksum[:])
		str2 := hex.EncodeToString(checkSum[:])
		log.Warning(fmt.Sprintf("Message Checksum error, Received checksum %s Wanted checksum: %s",
			str1, str2))
		return errors.New("Message Checksum error")
	}

	return nil
}

func (msg *msgHdr) Deserialize(r io.Reader) error {
	return binary.Read(r, binary.LittleEndian, msg)
}

func (hdr msgHdr) Serialize(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, hdr)
}

func (hdr msgHdr) Handle(n *RemoteNode) error {
	// TBD
	return nil
}

// Transaction message
type trn struct {
	msgHdr
	txn transaction.Transaction
}

func (msg trn) Handle(node *RemoteNode) error {
	txn := &msg.txn
	if !node.localNode.ExistHash(txn.Hash()) {
		// add transaction to pool when in consensus state
		if node.localNode.GetSyncState() == pb.PersistFinished {
			if errCode := node.localNode.AppendTxnPool(txn); errCode != nknErrors.ErrNoError {
				return fmt.Errorf("[message] VerifyTransaction failed with %v when AppendTxnPool.", errCode)
			}
		}
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
