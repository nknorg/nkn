package message

import (
	"bytes"
	"errors"

	. "github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/log"
)

type verACK struct {
	msgHdr
	// No payload
}

func NewVerack() ([]byte, error) {
	var msg verACK
	// Fixme the check is the []byte{0} instead of 0
	var sum []byte
	sum = []byte{0x5d, 0xf6, 0xe0, 0xe2}
	msg.msgHdr.init("verack", sum, 0)

	buff := bytes.NewBuffer(nil)
	err := msg.Serialize(buff)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

/*
 * The node state switch table after rx message, there is time limitation for each action
 * The Hanshake status will switch to INIT after TIMEOUT if not received the VerACK
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
 *
 */
// TODO The process should be adjusted based on above table
func (msg verACK) Handle(node Noder) error {
	s := node.GetState()
	if s != HANDSHAKE && s != HANDSHAKED {
		log.Warn("Unknown status to received verack")
		return errors.New("Unknown status to received verack")
	}

	node.SetState(ESTABLISH)

	if s == HANDSHAKE {
		buf, _ := NewVerack()
		node.Tx(buf)
	}

	node.DumpInfo()

	return nil
}
