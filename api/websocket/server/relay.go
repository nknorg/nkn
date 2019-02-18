package server

import (
	"encoding/hex"
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/util/log"
)

func ResolveDest(Dest string) string {
	substrings := strings.Split(Dest, ".")
	pubKeyOrName := substrings[len(substrings)-1]

	registrant, err := chain.DefaultLedger.Store.GetRegistrant(pubKeyOrName)
	if err != nil {
		return Dest
	}
	pubKeyStr := hex.EncodeToString(registrant)

	substrings[len(substrings)-1] = pubKeyStr
	return strings.Join(substrings, ".")
}

func (ws *WsServer) sendOutboundRelayMessage(srcAddrStrPtr *string, msg *pb.OutboundMessage) {
	if srcAddrStrPtr == nil {
		log.Error("src addr is nil")
		return
	}

	var signature []byte
	for _, dest := range append(msg.Dests, msg.Dest) {
		dest = ResolveDest(dest)

		err := ws.localNode.SendRelayMessage(*srcAddrStrPtr, dest, msg.Payload, signature, msg.MaxHoldingSeconds)
		if err != nil {
			log.Error("Send relay message error:", err)
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

func (ws *WsServer) sendInboundRelayMessage(relayMessage *pb.Relay) {
	clientID := relayMessage.DestId
	msg := &pb.InboundMessage{
		Src:     relayMessage.SrcAddr,
		Payload: relayMessage.Payload,
	}

	success := ws.sendInboundMessage(hex.EncodeToString(clientID), msg)
	if success {
		ws.localNode.GetEvent("relay").Notify(events.EventReceiveClientSignedSigChain, relayMessage)
	} else {
		ws.messageBuffer.AddMessage(clientID, relayMessage)
	}
}
