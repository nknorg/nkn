package message

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/log"
)

type RelayPacket struct {
	SrcAddr  string
	DestID   []byte
	Payload  []byte
	SigChain *por.SigChain
}

type RelayMessage struct {
	msgHdr
	packet RelayPacket
}

func NewRelayPacket(srcAddr string, destID, payload []byte, sigChain *por.SigChain) (*RelayPacket, error) {
	relayPakcet := &RelayPacket{
		SrcAddr:  srcAddr,
		DestID:   destID,
		Payload:  payload,
		SigChain: sigChain,
	}
	return relayPakcet, nil
}

func NewRelayMessage(packet *RelayPacket) (*RelayMessage, error) {
	var msg RelayMessage
	msg.msgHdr.Magic = NetID
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

	return &msg, nil
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

func (msg *RelayMessage) ToBytes() ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	err := msg.Serialize(buffer)
	if err != nil {
		log.Error("Error serializing relay message ", err.Error())
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (packet *RelayPacket) Serialize(w io.Writer) error {
	var err error
	err = serialization.WriteVarString(w, packet.SrcAddr)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, packet.DestID)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, packet.Payload)
	if err != nil {
		return err
	}

	buf, err := proto.Marshal(packet.SigChain)
	if err != nil {
		return err
	}
	err = serialization.WriteVarBytes(w, buf)
	if err != nil {
		return err
	}

	return nil
}

func (packet *RelayPacket) Deserialize(r io.Reader) error {
	srcAddr, err := serialization.ReadVarString(r)
	if err != nil {
		return err
	}
	packet.SrcAddr = srcAddr

	destID, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	packet.DestID = destID

	payload, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	packet.Payload = payload

	buf, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	packet.SigChain = &por.SigChain{}
	err = proto.Unmarshal(buf, packet.SigChain)
	if err != nil {
		return err
	}

	return nil
}
