package server

import (
	"encoding/hex"
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/node"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/util/log"
)

func ResolveDest(Dest string) string {
	substrings := strings.Split(Dest, ".")
	pubKeyOrName := substrings[len(substrings)-1]

	registrant, err := ledger.DefaultLedger.Store.GetRegistrant(pubKeyOrName)
	if err != nil {
		return Dest
	}
	pubKeyStr := hex.EncodeToString(registrant)

	substrings[len(substrings)-1] = pubKeyStr
	return strings.Join(substrings, ".")
}

func (ws *WsServer) sendOutboundRelayPacket(clientId string, msg *pb.OutboundMessage) {
	clients := ws.SessionList.GetSessionsById(clientId)
	if clients == nil {
		log.Error("Session not found")
		return
	}
	client := clients[0]
	if !client.IsClient() {
		log.Error("Session is not client")
		return
	}
	srcAddrStrPtr := client.GetAddrStr()
	if srcAddrStrPtr == nil {
		log.Error("Cannot get sender address")
		return
	}
	srcAddrStr := *srcAddrStrPtr

	var signature []byte
	for _, dest := range append(msg.Dests, msg.Dest) {
		dest = ResolveDest(dest)

		err := ws.localNode.SendRelayPacket(srcAddrStr, dest, msg.Payload, signature, msg.MaxHoldingSeconds)
		if err != nil {
			log.Error("Send relay packet error:", err)
		}
	}
}

func (ws *WsServer) sendInboundMessage(clientID string, msg *pb.InboundMessage) bool {
	clients := ws.SessionList.GetSessionsById(clientID)
	if clients == nil {
		log.Info("Client Not Online:", clientID)
		return false
	}

	buf, err := proto.Marshal(msg)
	if err != nil {
		log.Error(err)
		return false
	}

	success := false
	for _, client := range clients {
		if !client.IsClient() {
			log.Error("Session is not client")
			continue
		}

		err = client.SendBinary(buf)
		if err != nil {
			log.Error("Send to client error: ", err)
			continue
		}

		success = true
	}

	return success
}

func (ws *WsServer) sendInboundRelayPacket(packet *node.RelayPacket) {
	clientID := packet.DestID
	msg := &pb.InboundMessage{
		Src:     packet.SrcAddr,
		Payload: packet.Payload,
	}

	success := ws.sendInboundMessage(hex.EncodeToString(clientID), msg)
	if success {
		ws.localNode.GetEvent("relay").Notify(events.EventReceiveClientSignedSigChain, packet.SigChain)
	} else {
		ws.packetBuffer.AddPacket(clientID, packet)
	}
}
