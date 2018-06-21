package message

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"

	. "github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/log"
)

type addrReq struct {
	Hdr msgHdr // No payload
}

type addr struct {
	hdr       msgHdr
	nodeCnt   uint64
	nodeAddrs []NodeAddr
}

func newGetAddr() ([]byte, error) {
	var msg addrReq
	sum := []byte{0x5d, 0xf6, 0xe0, 0xe2}
	msg.Hdr.init("getaddr", sum, 0)

	buf := bytes.NewBuffer(nil)
	err := msg.Serialize(buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), err
}

func NewAddrs(nodeaddrs []NodeAddr, count uint64) ([]byte, error) {
	var msg addr
	msg.nodeAddrs = nodeaddrs
	msg.nodeCnt = count
	msg.hdr.Magic = NetID
	cmd := "addr"
	copy(msg.hdr.CMD[0:7], cmd)
	p := new(bytes.Buffer)
	err := binary.Write(p, binary.LittleEndian, msg.nodeCnt)
	if err != nil {
		log.Error("Binary Write failed at new Msg: ", err.Error())
		return nil, err
	}

	err = binary.Write(p, binary.LittleEndian, msg.nodeAddrs)
	if err != nil {
		log.Error("Binary Write failed at new Msg: ", err.Error())
		return nil, err
	}
	s := sha256.Sum256(p.Bytes())
	s2 := s[:]
	s = sha256.Sum256(s2)
	buf := bytes.NewBuffer(s[:4])
	binary.Read(buf, binary.LittleEndian, &(msg.hdr.Checksum))
	msg.hdr.Length = uint32(len(p.Bytes()))

	buff := bytes.NewBuffer(nil)
	err = msg.Serialize(buff)
	if err != nil {
		log.Error("Error Convert net message ", err.Error())
		return nil, err
	}

	return buff.Bytes(), nil
}

func (msg addrReq) Verify(buf []byte) error {
	return msg.Hdr.Verify(buf)
}

func (msg addrReq) Handle(node Noder) error {
	var addrstr []NodeAddr
	var count uint64
	addrstr, count = node.LocalNode().GetNeighborAddrs()
	buf, err := NewAddrs(addrstr, count)
	if err != nil {
		return err
	}
	go node.Tx(buf)
	return nil
}

func (msg addrReq) Serialize(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, msg)
}

func (msg *addrReq) Deserialize(r io.Reader) error {
	return binary.Read(r, binary.LittleEndian, msg)
}

func (msg addr) Serialize(w io.Writer) error {
	err := binary.Write(w, binary.LittleEndian, msg.hdr)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, msg.nodeCnt)
	if err != nil {
		return err
	}
	for _, v := range msg.nodeAddrs {
		err = binary.Write(w, binary.LittleEndian, v)
		if err != nil {
			return err
		}
	}

	return nil
}

func (msg *addr) Deserialize(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &(msg.hdr))
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &(msg.nodeCnt))
	if err != nil {
		return err
	}
	msg.nodeAddrs = make([]NodeAddr, msg.nodeCnt)
	for i := 0; i < int(msg.nodeCnt); i++ {
		err := binary.Read(r, binary.LittleEndian, &(msg.nodeAddrs[i]))
		if err != nil {
			return err
		}
	}

	return nil
}

func (msg addr) Verify(buf []byte) error {
	return msg.hdr.Verify(buf)
}

func (msg addr) Handle(node Noder) error {
	for _, v := range msg.nodeAddrs {
		var ip net.IP
		ip = v.IpAddr[:]
		address := ip.To16().String() + ":" + strconv.Itoa(int(v.Port))
		log.Info(fmt.Sprintf("The ip address is %s id is 0x%x", address, v.ID))

		if v.ID == node.LocalNode().GetID() {
			continue
		}
		if node.LocalNode().NodeEstablished(v.ID) {
			continue
		}

		if v.Port == 0 {
			continue
		}

		go node.LocalNode().Connect(address)
	}
	return nil
}
