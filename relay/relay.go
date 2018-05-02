package relay

import (
	"bytes"
	"errors"

	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/ws"
)

type RelayService struct {
	localNode        protocol.Noder    // local node
	relayMsgReceived events.Subscriber // consensus events listening
}

func NewRelayService(node protocol.Noder) *RelayService {
	service := &RelayService{
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
		ws.GetServer().Broadcast(packet.PayloadData)
		log.Info("Received packet from %x to %x with payload %s\n", packet.SrcID, destID, packet.PayloadData)
		return nil
	}
	nextHop, err := p.localNode.NextHop(destID)
	if err != nil {
		log.Error("Get next hop error: ", err)
		return err
	}
	if nextHop == nil {
		log.Errorf("No next hop for packet from %x to %x with payload %x\n", packet.SrcID, destID, packet.PayloadData)
		return nil
	}
	b, err := message.NewRelayMessage(packet)
	if err != nil {
		log.Error("Create relay message error: ", err)
		return err
	}
	log.Infof("Relay packet from %x to %x with payload %x\n", packet.SrcID, destID, packet.PayloadData)
	nextHop.Tx(b)
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
