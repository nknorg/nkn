package server

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/log"
)

type sigChainInfo struct {
	blockHash   []byte
	sigChainLen int
}

func ResolveDest(Dest string) string {
	substrings := strings.Split(Dest, ".")
	pubKeyOrName := substrings[len(substrings)-1]

	registrant, _, err := chain.DefaultLedger.Store.GetRegistrant(pubKeyOrName)
	if err != nil || registrant == nil {
		return Dest
	}
	pubKeyStr := hex.EncodeToString(registrant)

	substrings[len(substrings)-1] = pubKeyStr
	return strings.Join(substrings, ".")
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
		dest = ResolveDest(dest)
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
		log.Infof("Client Not Online: %s", clientID)
		return false
	}

	buf, err := proto.Marshal(inboundMsg)
	if err != nil {
		log.Errorf("Marshal inbound message error: %v", err)
		return false
	}

	msg := &pb.ClientMessage{
		MessageType: pb.INBOUND_MESSAGE,
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

func (ws *WsServer) sendInboundRelayMessage(relayMessage *pb.Relay) {
	clientID := relayMessage.DestId
	msg := &pb.InboundMessage{
		Src:     address.AssembleClientAddress(relayMessage.SrcIdentifier, relayMessage.SrcPubkey),
		Payload: relayMessage.Payload,
	}

	shouldSign := por.GetPorServer().ShouldSignDestSigChainElem(relayMessage.BlockHash, relayMessage.LastSignature, int(relayMessage.SigChainLen))
	if shouldSign {
		msg.PrevSignature = relayMessage.LastSignature
	}

	success := ws.sendInboundMessage(hex.EncodeToString(clientID), msg)
	if success {
		if shouldSign {
			ws.sigChainCache.Add(relayMessage.LastSignature, &sigChainInfo{
				blockHash:   relayMessage.BlockHash,
				sigChainLen: int(relayMessage.SigChainLen),
			})
		}
	} else {
		ws.messageBuffer.AddMessage(clientID, relayMessage)
	}
}

func (ws *WsServer) handleReceipt(receipt *pb.Receipt) error {
	v, ok := ws.sigChainCache.Get(receipt.PrevSignature)
	if !ok {
		return fmt.Errorf("sigchain info with last signature %x not found in cache", receipt.PrevSignature)
	}

	sci, ok := v.(*sigChainInfo)
	if !ok {
		return errors.New("convert to sigchain info failed")
	}

	if !por.GetPorServer().ShouldSignDestSigChainElem(sci.blockHash, receipt.PrevSignature, sci.sigChainLen) {
		return nil
	}

	destSigChainElem := pb.NewSigChainElem(nil, nil, receipt.Signature, nil, nil, false, pb.SIGNATURE)
	por.GetPorServer().AddDestSigChainElem(
		sci.blockHash,
		receipt.PrevSignature,
		sci.sigChainLen,
		destSigChainElem,
	)

	return nil
}
