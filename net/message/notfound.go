package message

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"

	"github.com/nknorg/nkn/common"
	. "github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/log"
)

type notFound struct {
	msgHdr
	hash common.Uint256
}

func NewNotFound(hash common.Uint256) ([]byte, error) {
	var msg notFound
	msg.hash = hash
	msg.msgHdr.Magic = NetID
	cmd := "notfound"
	copy(msg.msgHdr.CMD[0:len(cmd)], cmd)
	tmpBuffer := bytes.NewBuffer([]byte{})
	msg.hash.Serialize(tmpBuffer)
	p := new(bytes.Buffer)
	err := binary.Write(p, binary.LittleEndian, tmpBuffer.Bytes())
	if err != nil {
		log.Error("Binary Write failed at new notfound Msg")
		return nil, err
	}
	s := sha256.Sum256(p.Bytes())
	s2 := s[:]
	s = sha256.Sum256(s2)
	buf := bytes.NewBuffer(s[:4])
	binary.Read(buf, binary.LittleEndian, &(msg.msgHdr.Checksum))
	msg.msgHdr.Length = uint32(len(p.Bytes()))

	notfoundBuff := bytes.NewBuffer(nil)
	err = msg.Serialize(notfoundBuff)
	if err != nil {
		log.Error("Error Convert net message ", err.Error())
		return nil, err
	}

	return notfoundBuff.Bytes(), nil
}

func (msg notFound) Verify(buf []byte) error {
	return msg.msgHdr.Verify(buf)
}

func (msg notFound) Serialize(w io.Writer) error {
	err := msg.msgHdr.Serialize(w)
	if err != nil {
		return err
	}
	_, err = msg.hash.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (msg *notFound) Deserialize(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &(msg.msgHdr))
	if err != nil {
		log.Warn("Parse notfound message hdr error")
		return errors.New("Parse notfound message hdr error")
	}
	err = msg.hash.Deserialize(r)
	if err != nil {
		log.Warn("Parse notfound message error")
		return errors.New("Parse notfound message error")
	}

	return nil
}

func (msg notFound) Handle(node Noder) error {
	return nil
}
