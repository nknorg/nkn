package server

import (
	"encoding/hex"
	"strings"

	"github.com/nknorg/nkn/api/websocket/client"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/util/log"
)

func ResolveDest(Dest string) string {
	substrings := strings.Split(Dest, ".")
	pubKeyStr := substrings[len(substrings)-1]
	registrant, err := ledger.DefaultLedger.Store.GetRegistrant(pubKeyStr)
	if err != nil {
		return Dest
	}

	pubKeyStr = hex.EncodeToString(registrant)
	if len(substrings) > 1 {
		identifier := substrings[0]
		return strings.Join([]string{identifier, pubKeyStr}, ".")
	} else {
		return pubKeyStr
	}
}

func (ws *WsServer) SendRelayPacket(clientId string, msg *client.OutboundMessage) {
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

	msg.Dest = ResolveDest(msg.Dest)

	var signature []byte
	for _, dest := range append(msg.Dests, msg.Dest) {
		err := ws.node.SendRelayPacket(srcAddrStr, dest, msg.Payload, signature)
		if err != nil {
			log.Error("Send relay packet error:", err)
		}
	}
}
