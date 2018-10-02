package server

import (
	"github.com/nknorg/nkn/api/websocket/client"
	"github.com/nknorg/nkn/util/log"
)

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
	var signature []byte
	for _, dest := range append(msg.Dests, msg.Dest) {
		err := ws.node.SendRelayPacket(srcAddrStr, dest, msg.Payload, signature)
		if err != nil {
			log.Error("Send relay packet error:", err)
		}
	}
}
