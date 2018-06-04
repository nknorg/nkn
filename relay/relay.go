package relay

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/rpc/httpjson"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
	"github.com/nknorg/nkn/websocket"
)

type RelayService struct {
	wallet           wallet.Wallet     // wallet
	localNode        protocol.Noder    // local node
	porServer        *por.PorServer    // por server to handle signature chain
	relayMsgReceived events.Subscriber // consensus events listening
}

func NewRelayService(wallet wallet.Wallet, node protocol.Noder) *RelayService {
	service := &RelayService{
		wallet:    wallet,
		localNode: node,
		porServer: por.GetPorServer(),
	}
	return service
}

func (rs *RelayService) Start() error {
	rs.relayMsgReceived = rs.localNode.GetEvent("relay").Subscribe(events.EventRelayMsgReceived, rs.ReceiveRelayMsgNoError)
	return nil
}

func (rs *RelayService) SendPacketToClient(client Client, packet *message.RelayPacket) error {
	destPubKey := packet.SigChain.GetDestPubkey()
	if !bytes.Equal(client.GetPubKey(), destPubKey) {
		return errors.New("Client pubkey is different from destination pubkey")
	}
	err := rs.porServer.Sign(packet.SigChain, destPubKey)
	if err != nil {
		log.Error("Signing signature chain error: ", err)
		return err
	}

	// TODO: only pick sigchain to sign when threshold is smaller than

	digest, err := packet.SigChain.ExtendElement(destPubKey)
	if err != nil {
		return err
	}
	response := map[string]interface{}{
		"Action":  "receivePacket",
		"Src":     string(packet.SrcID),
		"Payload": string(packet.Payload),
		"Digest":  digest,
	}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return err
	}
	err = client.Send(responseJSON)
	if err != nil {
		log.Error("Send to client error: ", err)
		return err
	}

	// TODO: create and send tx only after client sign
	buf, err := proto.Marshal(packet.SigChain)
	txn, err := httpjson.MakeCommitTransaction(rs.wallet, buf)
	if err != nil {
		return err
	}
	rs.localNode.Xmit(txn)

	return nil
}

func (rs *RelayService) SendPacketToNode(nextHop protocol.Noder, packet *message.RelayPacket) error {
	nextPubkey, err := nextHop.GetPubKey().EncodePoint(true)
	if err != nil {
		log.Error("Get next hop public key error: ", err)
		return err
	}
	err = rs.porServer.Sign(packet.SigChain, nextPubkey)
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
		"Relay packet:\nSrcID: %x\nDestID: %x\nNext Hop: %s:%d\nPayload %x",
		packet.SrcID,
		packet.DestID,
		nextHop.GetAddr(),
		nextHop.GetPort(),
		packet.Payload,
	)
	nextHop.Tx(msgBytes)
	return nil
}

func (rs *RelayService) HandleMsg(packet *message.RelayPacket) error {
	destID := packet.DestID
	if bytes.Equal(rs.localNode.GetChordAddr(), destID) {
		log.Infof(
			"Receive packet:\nSrcID: %x\nDestID: %x\nPayload %x",
			packet.SrcID,
			destID,
			packet.Payload,
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
		client := websocket.GetServer().GetClientById(destID)
		if client == nil {
			// TODO: handle client not exists
			return errors.New("Client Not Exists: " + hex.EncodeToString(destID))
		}
		err = rs.SendPacketToClient(client, packet)
		if err != nil {
			return err
		}
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
