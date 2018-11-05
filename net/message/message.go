package message

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/nknorg/nkn/common/serialization"
	. "github.com/nknorg/nkn/net/protocol"
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
	Handle(Noder) error
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
	case "getheaders":
		var msg headersReq
		// TODO fill the header and type
		copy(msg.hdr.CMD[0:len(t)], t)
		return &msg
	case "headers":
		var msg blkHeader
		copy(msg.hdr.CMD[0:len(t)], t)
		return &msg
	case "getdata":
		var msg dataReq
		copy(msg.msgHdr.CMD[0:len(t)], t)
		return &msg
	case "block":
		var msg block
		copy(msg.msgHdr.CMD[0:len(t)], t)
		return &msg
	case "tx":
		var msg trn
		copy(msg.msgHdr.CMD[0:len(t)], t)
		return &msg
	case "ising":
		var msg IsingMessage
		copy(msg.msgHdr.CMD[0:len(t)], t)
		return &msg
	case "notfound":
		var msg notFound
		copy(msg.msgHdr.CMD[0:len(t)], t)
		return &msg
	case "relay":
		var msg RelayMessage
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

func HandleNodeMsg(node Noder, buf []byte) error {
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

func (hdr msgHdr) Handle(n Noder) error {
	// TBD
	return nil
}
