package relay

import (
	"bytes"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/api/common"
	"github.com/nknorg/nkn/api/websocket"
	"github.com/nknorg/nkn/api/websocket/client"
	"github.com/nknorg/nkn/api/websocket/session"
	nknErrors "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
)

type RelayService struct {
	sync.Mutex
	wallet            vault.Wallet                      // wallet
	localNode         protocol.Noder                    // local node
	porServer         *por.PorServer                    // por server to handle signature chain
	relayMsgReceived  events.Subscriber                 // consensus events listening
	relayPacketBuffer map[string][]*message.RelayPacket // for offline clients
}

func NewRelayService(wallet vault.Wallet, node protocol.Noder) *RelayService {
	service := &RelayService{
		wallet:            wallet,
		localNode:         node,
		porServer:         por.GetPorServer(),
		relayPacketBuffer: make(map[string][]*message.RelayPacket),
	}
	return service
}

func (rs *RelayService) Start() error {
	rs.relayMsgReceived = rs.localNode.GetEvent("relay").Subscribe(events.EventRelayMsgReceived, rs.ReceiveRelayMsgNoError)
	return nil
}

func (rs *RelayService) SendPacketToClients(clients []*session.Session, packet *message.RelayPacket) error {
	if len(clients) == 0 {
		return nil
	}

	destPubKey := packet.SigChain.GetDestPubkey()
	for _, client := range clients {
		if !bytes.Equal(client.GetPubKey(), destPubKey) {
			return errors.New("Client pubkey is different from destination pubkey")
		}
	}

	mining := false
	if rs.localNode.GetSyncState() == protocol.PersistFinished {
		mining = true
	}
	err := rs.porServer.Sign(packet.SigChain, destPubKey, mining)
	if err != nil {
		log.Error("Signing signature chain error: ", err)
		return err
	}

	// TODO: only pick sigchain to sign when threshold is smaller than

	_, err = packet.SigChain.ExtendElement(destPubKey, false)
	if err != nil {
		return err
	}
	msg := &client.InboundMessage{
		Src:     packet.SrcAddr,
		Payload: packet.Payload,
	}
	buf, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	ok := false
	for _, client := range clients {
		err = client.SendBinary(buf)
		if err != nil {
			log.Error("Send to client error: ", err)
		} else {
			ok = true
		}
	}

	if !ok {
		return errors.New("Send packet to clients all failed")
	}

	// TODO: create and send tx only after client sign
	buf, err = proto.Marshal(packet.SigChain)
	if err != nil {
		return err
	}
	txn, err := common.MakeCommitTransaction(rs.wallet, buf)
	if err != nil {
		return err
	}
	errCode := rs.localNode.AppendTxnPool(txn)
	if errCode != nknErrors.ErrNoError {
		return errCode
	}
	err = rs.localNode.Xmit(txn)
	if err != nil {
		return err
	}

	return nil
}

func (rs *RelayService) SendPacketToNode(nextHop protocol.Noder, packet *message.RelayPacket) error {
	nextPubkey, err := nextHop.GetPubKey().EncodePoint(true)
	if err != nil {
		log.Error("Get next hop public key error: ", err)
		return err
	}
	mining := false
	if rs.localNode.GetSyncState() == protocol.PersistFinished {
		mining = true
	}
	err = rs.porServer.Sign(packet.SigChain, nextPubkey, mining)
	if err != nil {
		log.Error("Signing signature chain error: ", err)
		return err
	}
	msg, err := message.NewRelayMessage(packet)
	if err != nil {
		log.Error("Create relay message error: ", err)
		return err
	}
	msgBytes, err := msg.ToBytes()
	if err != nil {
		log.Error("Convert relay message to bytes error: ", err)
		return err
	}
	log.Infof(
		"Relay packet:\nSrcID: %s\nDestID: %x\nNext Hop: %s:%d\nPayload Size: %d",
		packet.SrcAddr,
		packet.DestID,
		nextHop.GetAddr(),
		nextHop.GetPort(),
		len(packet.Payload),
	)
	nextHop.Tx(msgBytes)
	return nil
}

func (rs *RelayService) HandleMsg(packet *message.RelayPacket) error {
	destID := packet.DestID
	if bytes.Equal(rs.localNode.GetChordAddr(), destID) {
		log.Infof(
			"Receive packet:\nSrcID: %s\nDestID: %x\nPayload Size: %d",
			packet.SrcAddr,
			destID,
			len(packet.Payload),
		)
		// TODO: handle packet send to self
		return nil
	}
	nextHop, err := rs.localNode.NextHop(destID)
	if err != nil {
		log.Error("Get next hop error: ", err)
		return err
	}
	if nextHop == nil {
		clients := websocket.GetServer().GetClientsById(destID)
		if clients == nil {
			log.Info("Client Not Online:", hex.EncodeToString(destID))
			rs.addRelayPacketToBuffer(destID, packet)
			return nil
		}
		rs.SendPacketToClients(clients, packet)
		return nil
	}
	err = rs.SendPacketToNode(nextHop, packet)
	if err != nil {
		return err
	}
	return nil
}

func (rs *RelayService) ReceiveRelayMsg(v interface{}) error {
	if packet, ok := v.(*message.RelayPacket); ok {
		return rs.HandleMsg(packet)
	} else {
		return errors.New("Decode relay msg failed")
	}
}

func (rs *RelayService) ReceiveRelayMsgNoError(v interface{}) {
	err := rs.ReceiveRelayMsg(v)
	if err != nil {
		log.Error(err.Error())
	}
}

func (rs *RelayService) addRelayPacketToBuffer(clientID []byte, packet *message.RelayPacket) {
	clientIDStr := hex.EncodeToString(clientID)
	rs.Lock()
	defer rs.Unlock()
	rs.relayPacketBuffer[clientIDStr] = append(rs.relayPacketBuffer[clientIDStr], packet)
}

func (rs *RelayService) SendRelayPacketsInBuffer(clientID []byte) error {
	clientIDStr := hex.EncodeToString(clientID)
	clients := websocket.GetServer().GetClientsById(clientID)
	if clients == nil {
		return nil
	}

	rs.Lock()
	defer rs.Unlock()
	packets := rs.relayPacketBuffer[clientIDStr]
	if len(packets) == 0 {
		return nil
	}

	for _, packet := range packets {
		rs.SendPacketToClients(clients, packet)
	}
	rs.relayPacketBuffer[clientIDStr] = nil
	return nil
}
