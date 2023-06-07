package server

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/por"
	"github.com/nknorg/nkn/v2/util/address"
	"github.com/nknorg/nkn/v2/util/log"
)

type sigChainInfo struct {
	blockHash   []byte
	sigChainLen int
}

func (ws *WsServer) sendOutboundRelayMessage(srcAddrStrPtr *string, msg *pb.OutboundMessage) {
	if srcAddrStrPtr == nil {
		log.Warningf("src addr is nil")
		return
	}

	dests := msg.Dests
	if len(dests) == 0 && len(msg.Dest) > 0 {
		dests = append(dests, msg.Dest)
	}

	if len(dests) == 0 {
		log.Warningf("no destination")
		return
	}

	payloads := msg.Payloads
	if len(payloads) == 0 && len(msg.Payload) > 0 {
		payloads = append(payloads, msg.Payload)
	}

	if len(payloads) == 0 {
		log.Warningf("no payload")
		return
	}

	if len(payloads) > 1 && len(payloads) != len(dests) {
		log.Warningf("payloads length %d is different from dests length %d", len(payloads), len(dests))
		return
	}

	var payload []byte
	for i, dest := range dests {
		if len(payloads) > 1 {
			payload = payloads[i]
		} else {
			payload = payloads[0]
		}
		err := ws.localNode.SendRelayMessage(*srcAddrStrPtr, dest, payload, msg.Signatures[i], msg.BlockHash, msg.Nonce, msg.MaxHoldingSeconds)
		if err != nil {
			log.Error("Send relay message error:", err)
		}
	}
}

func (ws *WsServer) sendInboundMessage(clientID string, inboundMsg *pb.InboundMessage) bool {
	clients := ws.SessionList.GetSessionsById(clientID)
	if clients == nil {
		log.Debugf("Client Not Online: %s", clientID)
		return false
	}

	buf, err := proto.Marshal(inboundMsg)
	if err != nil {
		log.Errorf("Marshal inbound message error: %v", err)
		return false
	}

	msg := &pb.ClientMessage{
		MessageType: pb.ClientMessageType_INBOUND_MESSAGE,
		Message:     buf,
	}
	buf, err = proto.Marshal(msg)
	if err != nil {
		log.Errorf("Marshal client message error: %v", err)
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

func (ws *WsServer) sendInboundRelayMessage(relayMessage *pb.Relay, shouldSign bool) {
	clientID := relayMessage.DestId
	msg := &pb.InboundMessage{
		Src:     address.AssembleClientAddress(relayMessage.SrcIdentifier, relayMessage.SrcPubkey),
		Payload: relayMessage.Payload,
	}

	if shouldSign {
		shouldSign = por.GetPorServer().ShouldSignDestSigChainElem(relayMessage.BlockHash, relayMessage.LastHash, int(relayMessage.SigChainLen))
		if shouldSign {
			msg.PrevHash = relayMessage.LastHash
		}
	}

	success := ws.sendInboundMessage(hex.EncodeToString(clientID), msg)
	if success {
		if shouldSign {
			ws.sigChainCache.Add(relayMessage.LastHash, &sigChainInfo{
				blockHash:   relayMessage.BlockHash,
				sigChainLen: int(relayMessage.SigChainLen),
			})
		}
		if time.Duration(relayMessage.MaxHoldingSeconds) > pongTimeout/time.Second {
			ok := ws.messageDeliveredCache.Push(relayMessage)
			if !ok {
				log.Warningf("MessageDeliveredCache full, discarding messages.")
			}
		}
	} else if relayMessage.MaxHoldingSeconds > 0 {
		ws.messageBuffer.AddMessage(clientID, relayMessage)
	}
}

func (ws *WsServer) startCheckingLostMessages() {
	for {
		v, ok := ws.messageDeliveredCache.Pop()
		if !ok {
			break
		}
		if relayMessage, ok := v.(*pb.Relay); ok {
			clientID := relayMessage.DestId
			clients := ws.SessionList.GetSessionsById(hex.EncodeToString(clientID))
			if len(clients) > 0 {
				threshold := time.Now().Add(-pongTimeout)
				success := false
				for _, client := range clients {
					if client.GetLastReadTime().After(threshold) && client.GetConnectTime().Before(threshold) {
						success = true
						break
					}
				}
				if !success {
					ws.sendInboundRelayMessage(relayMessage, false)
				}
				continue
			}
			ws.messageBuffer.AddMessage(clientID, relayMessage)
		}
	}
}

func (ws *WsServer) handleReceipt(receipt *pb.Receipt) error {
	v, ok := ws.sigChainCache.Get(receipt.PrevHash)
	if !ok {
		return fmt.Errorf("sigchain info with last hash %x not found in cache", receipt.PrevHash)
	}

	sci, ok := v.(*sigChainInfo)
	if !ok {
		return errors.New("convert to sigchain info failed")
	}

	if !por.GetPorServer().ShouldSignDestSigChainElem(sci.blockHash, receipt.PrevHash, sci.sigChainLen) {
		return nil
	}

	destSigChainElem := pb.NewSigChainElem(nil, nil, receipt.Signature, nil, nil, false, pb.SigAlgo_SIGNATURE)
	por.GetPorServer().AddDestSigChainElem(
		sci.blockHash,
		receipt.PrevHash,
		sci.sigChainLen,
		destSigChainElem,
	)

	return nil
}
