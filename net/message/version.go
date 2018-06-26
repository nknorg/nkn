package message

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/crypto"
	. "github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

const (
	HTTPINFOFLAG = 0
)

type version struct {
	Hdr msgHdr
	P   struct {
		Version      uint32
		Services     uint64
		TimeStamp    uint32
		Port         uint16
		HttpInfoPort uint16
		Cap          [32]byte
		Nonce        uint64
		// TODO remove tempory to get serilization function passed
		UserAgent   uint8
		StartHeight uint32
		// FIXME check with the specify relay type length
		Relay uint8
	}
	pk *crypto.PubKey
}

func (msg *version) init(n Noder) {
	// Do the init
}

func NewVersion(n Noder) ([]byte, error) {
	var msg version
	msg.P.Version = n.Version()
	msg.P.Services = n.Services()
	msg.P.HttpInfoPort = config.Parameters.HttpInfoPort
	if config.Parameters.HttpInfoStart {
		msg.P.Cap[HTTPINFOFLAG] = 0x01
	} else {
		msg.P.Cap[HTTPINFOFLAG] = 0x00
	}

	// FIXME Time overflow
	msg.P.TimeStamp = uint32(time.Now().UTC().UnixNano())
	msg.P.Port = n.GetPort()
	msg.P.Nonce = n.GetID()
	msg.P.UserAgent = 0x00
	msg.P.StartHeight = ledger.DefaultLedger.GetLocalBlockChainHeight()
	if n.GetRelay() {
		msg.P.Relay = 1
	} else {
		msg.P.Relay = 0
	}

	msg.pk = n.GetBookKeeperAddr()
	// TODO the function to wrap below process
	// msg.HDR.init("version", n.GetID(), uint32(len(p.Bytes())))

	msg.Hdr.Magic = NetID
	copy(msg.Hdr.CMD[0:7], "version")
	p := bytes.NewBuffer([]byte{})
	err := binary.Write(p, binary.LittleEndian, &(msg.P))
	msg.pk.Serialize(p)
	if err != nil {
		log.Error("Binary Write failed at new Msg")
		return nil, err
	}
	s := sha256.Sum256(p.Bytes())
	s2 := s[:]
	s = sha256.Sum256(s2)
	buf := bytes.NewBuffer(s[:4])
	binary.Read(buf, binary.LittleEndian, &(msg.Hdr.Checksum))
	msg.Hdr.Length = uint32(len(p.Bytes()))

	versionBuff := bytes.NewBuffer(nil)
	err = msg.Serialize(versionBuff)
	if err != nil {
		log.Error("Error Convert net message ", err.Error())
		return nil, err
	}

	return versionBuff.Bytes(), nil
}

func (msg version) Verify(buf []byte) error {
	return msg.Hdr.Verify(buf)
}

func (msg version) Serialize(w io.Writer) error {
	err := msg.Hdr.Serialize(w)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, msg.P)
	if err != nil {
		return err
	}
	err = msg.pk.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (msg *version) Deserialize(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &(msg.Hdr))
	if err != nil {
		log.Warn("Parse version message hdr error")
		return errors.New("Parse version message hdr error")
	}

	err = binary.Read(r, binary.LittleEndian, &(msg.P))
	if err != nil {
		log.Warn("Parse version P message error")
		return errors.New("Parse version P message error")
	}

	pk := new(crypto.PubKey)
	err = pk.Deserialize(r)
	if err != nil {
		return errors.New("Parse pubkey Deserialize failed.")
	}
	msg.pk = pk

	return nil
}

/*
 * The node state switch table after rx message, there is time limitation for each action
 * The Handshake status will switch to INIT after TIMEOUT if not received the VerACK
 * in this time window
 *  _______________________________________________________________________
 * |          |    INIT         | HANDSHAKE |  ESTABLISH | INACTIVITY      |
 * |-----------------------------------------------------------------------|
 * | version  | HANDSHAKE(timer)|           |            | HANDSHAKE(timer)|
 * |          | if helloTime > 3| Tx verack | Depend on  | if helloTime > 3|
 * |          | Tx version      |           | node update| Tx version      |
 * |          | then Tx verack  |           |            | then Tx verack  |
 * |-----------------------------------------------------------------------|
 * | verack   |                 | ESTABLISH |            |                 |
 * |          |   No Action     |           | No Action  | No Action       |
 * |------------------------------------------------------------------------
 */
func (msg version) Handle(node Noder) error {
	localNode := node.LocalNode()

	// Exclude the node itself
	if msg.P.Nonce == localNode.GetID() {
		log.Warn("The node handshark with itself")
		node.CloseConn()
		return errors.New("The node handshark with itself")
	}

	s := node.GetState()
	if s != INIT && s != HAND {
		log.Warn("Unknown status to received version")
		return errors.New("Unknown status to received version")
	}

	// Obsolete node
	n, ret := localNode.DelNbrNode(msg.P.Nonce)
	if ret == true {
		log.Info(fmt.Sprintf("Node reconnect 0x%x", msg.P.Nonce))
		// Close the connection and release the node soure
		n.SetState(INACTIVITY)
		n.CloseConn()
	}

	if msg.P.Cap[HTTPINFOFLAG] == 0x01 {
		node.SetHttpInfoState(true)
	} else {
		node.SetHttpInfoState(false)
	}
	node.SetHttpInfoPort(msg.P.HttpInfoPort)
	node.SetBookKeeperAddr(msg.pk)
	node.UpdateInfo(time.Now(), msg.P.Version, msg.P.Services,
		msg.P.Port, msg.P.Nonce, msg.P.Relay, msg.P.StartHeight)
	localNode.AddNbrNode(node)

	var buf []byte
	if s == INIT {
		node.SetState(HANDSHAKE)
		buf, _ = NewVersion(localNode)
	} else if s == HAND {
		node.SetState(HANDSHAKED)
		buf, _ = NewVerack()
	}
	node.Tx(buf)

	return nil
}
