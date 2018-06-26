package message

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"io"

	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/ledger"
	. "github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/log"
)

type pong struct {
	msgHdr
	height uint32
}

func NewPongMsg() ([]byte, error) {
	var msg pong
	msg.msgHdr.Magic = NetID
	copy(msg.msgHdr.CMD[0:7], "pong")
	msg.height = ledger.DefaultLedger.Store.GetHeaderHeight()
	tmpBuffer := bytes.NewBuffer([]byte{})
	serialization.WriteUint32(tmpBuffer, msg.height)
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

	pongBuff := bytes.NewBuffer(nil)
	err = msg.Serialize(pongBuff)
	if err != nil {
		log.Error("Error Convert net message ", err.Error())
		return nil, err
	}

	return pongBuff.Bytes(), nil
}

func (msg pong) Verify(buf []byte) error {
	return msg.msgHdr.Verify(buf)
}

func (msg pong) Handle(node Noder) error {
	node.SetHeight(msg.height)
	return nil
}

func (msg pong) Serialize(w io.Writer) error {
	err := msg.msgHdr.Serialize(w)
	if err != nil {
		return err
	}
	err = serialization.WriteUint32(w, msg.height)
	if err != nil {
		return err
	}

	return nil
}

func (msg *pong) Deserialize(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &(msg.msgHdr))
	if err != nil {
		return err
	}
	msg.height, err = serialization.ReadUint32(r)
	if err != nil {
		return err
	}

	return nil
}
