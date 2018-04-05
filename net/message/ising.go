package message

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"io"

	. "nkn-core/net/protocol"
	"nkn-core/common/log"
	"nkn-core/common/serialization"
	"nkn-core/events"
)

type IsingPayload struct {
	PayloadData []byte
}

type IsingMessage struct {
	msgHdr
	pld IsingPayload
}


func (msg IsingMessage) Handle(node Noder) error {
	node.LocalNode().GetEvent("consensus").Notify(events.EventConsensusMsgReceived, &msg.pld)
	return nil
}

func (p *IsingMessage) Serialization() ([]byte, error) {
	msgHeader, err := p.msgHdr.Serialization()
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(msgHeader)
	err = p.pld.Serialize(buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), err
}


func (p *IsingMessage) Deserialization(b []byte) error {
	buf := bytes.NewBuffer(b)
	err := binary.Read(buf, binary.LittleEndian, &(p.msgHdr))
	err = p.pld.Deserialize(buf)

	return err
}


func (p *IsingPayload) Serialize(w io.Writer) error {
	err := serialization.WriteVarBytes(w, p.PayloadData)
	if err != nil {
		return err
	}

	return nil
}

func (p *IsingPayload) Deserialize(r io.Reader) error {
	pldData, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	p.PayloadData = pldData

	return nil
}

func NewIsingConsensus(pld *IsingPayload) ([]byte, error) {
	var msg IsingMessage
	msg.msgHdr.Magic = NETMAGIC
	cmd := "ising"
	copy(msg.msgHdr.CMD[0:len(cmd)], cmd)
	tmpBuffer := bytes.NewBuffer(nil)
	pld.Serialize(tmpBuffer)
	msg.pld = *pld
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

	m, err := msg.Serialization()
	if err != nil {
		log.Error("Error Convert net message ", err.Error())
		return nil, err
	}

	return m, nil
}
