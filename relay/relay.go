package relay

import (
	"bytes"
	"errors"

	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
	"github.com/nknorg/nkn/websocket"
)

type RelayService struct {
	account          *wallet.Account
	localNode        protocol.Noder    // local node
	relayMsgReceived events.Subscriber // consensus events listening
}

func NewRelayService(account *wallet.Account, node protocol.Noder) *RelayService {
	service := &RelayService{
		account:   account,
		localNode: node,
	}
	return service
}

func (p *RelayService) Start() error {
	p.relayMsgReceived = p.localNode.GetEvent("relay").Subscribe(events.EventRelayMsgReceived, p.ReceiveRelayMsgNoError)
	return nil
}

func (p *RelayService) HandleMsg(packet *message.RelayPacket) error {
	destID := packet.DestID
	if bytes.Equal(p.localNode.GetChordAddr(), destID) {
		log.Infof(
			"Receive packet:\nSrcID: %x\nDestID: %x\nPayload %x",
			packet.SrcID,
			destID,
			packet.Payload,
		)
		websocket.GetServer().Broadcast(packet.Payload)
		return nil
	}
	nextHop, err := p.localNode.NextHop(destID)
	if err != nil {
		log.Error("Get next hop error: ", err)
		return err
	}
	if nextHop == nil {
		log.Infof(
			"No next hop for packet:\nSrcID: %x\nDestID: %x\nPayload %x",
			packet.SrcID,
			destID,
			packet.Payload,
		)
		return nil
	}
	nextPubkey, err := nextHop.GetPubKey().EncodePoint(true)
	if err != nil {
		log.Error("Get next hop public key error: ", err)
		return err
	}
	packet.SigChain.Sign(nextPubkey, p.account)
	msg, err := message.NewRelayMessage(packet)
	if err != nil {
		log.Error("Create relay message error: ", err)
		return err
	}
	log.Infof(
		"Relay packet:\nSrcID: %x\nDestID: %x\nNext Hop: %s:%d\nPayload %x",
		packet.SrcID,
		destID,
		nextHop.GetAddr(),
		nextHop.GetPort(),
		packet.Payload,
	)
	nextHop.Tx(msg)
	return nil
}

func (p *RelayService) ReceiveRelayMsg(v interface{}) error {
	if packet, ok := v.(*message.RelayPacket); ok {
		return p.HandleMsg(packet)
	} else {
		return errors.New("Decode relay msg failed")
	}
}

func (p *RelayService) ReceiveRelayMsgNoError(v interface{}) {
	err := p.ReceiveRelayMsg(v)
	if err != nil {
		log.Error(err.Error())
	}
}

func (rs *RelayService) GetAccount() *wallet.Account {
	return rs.account
}
