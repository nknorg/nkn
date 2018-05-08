package message

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"io"

	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/log"
)

type RelayPacket struct {
	SrcID    []byte
	DestID   []byte
	SigChain *por.SigChain
	Payload  []byte
}

type RelayMessage struct {
	msgHdr
	packet RelayPacket
}

func (msg RelayMessage) Handle(node protocol.Noder) error {
	node.LocalNode().GetEvent("relay").Notify(events.EventRelayMsgReceived, &msg.packet)
	return nil
}

func (msg *RelayMessage) Serialize(w io.Writer) error {
	err := msg.msgHdr.Serialize(w)
	if err != nil {
		return err
	}

	err = msg.packet.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (msg *RelayMessage) Deserialize(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &(msg.msgHdr))
	if err != nil {
		return err
	}

	err = msg.packet.Deserialize(r)
	if err != nil {
		return err
	}

	return nil
}

func (packet *RelayPacket) Serialize(w io.Writer) error {
	var err error
	err = serialization.WriteVarBytes(w, packet.SrcID)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, packet.DestID)
	if err != nil {
		return err
	}

	err = packet.SigChain.Serialize(w)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, packet.Payload)
	if err != nil {
		return err
	}

	return nil
}

func (packet *RelayPacket) Deserialize(r io.Reader) error {
	srcID, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	packet.SrcID = srcID

	destID, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	packet.DestID = destID

	packet.SigChain = &por.SigChain{}
	err = packet.SigChain.Deserialize(r)
	if err != nil {
		return err
	}

	payload, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	packet.Payload = payload

	return nil
}

func NewRelayMessage(packet *RelayPacket) ([]byte, error) {
	var msg RelayMessage
	msg.msgHdr.Magic = protocol.NETMAGIC
	cmd := "relay"
	copy(msg.msgHdr.CMD[0:len(cmd)], cmd)
	tmpBuffer := bytes.NewBuffer(nil)
	packet.Serialize(tmpBuffer)
	msg.packet = *packet
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

	relayBuffer := bytes.NewBuffer(nil)
	err = msg.Serialize(relayBuffer)
	if err != nil {
		log.Error("Error Convert net message ", err.Error())
		return nil, err
	}

	return relayBuffer.Bytes(), nil
}
