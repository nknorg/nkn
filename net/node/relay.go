package node

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	nknErrors "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/vault"
	"github.com/nknorg/nnet/log"
)

type RelayService struct {
	sync.Mutex
	wallet    vault.Wallet
	localNode *LocalNode
	porServer *por.PorServer
}

func NewRelayService(wallet vault.Wallet, localNode *LocalNode) *RelayService {
	service := &RelayService{
		wallet:    wallet,
		localNode: localNode,
		porServer: por.GetPorServer(),
	}
	return service
}

func (rs *RelayService) Start() error {
	rs.localNode.GetEvent("relay").Subscribe(events.EventRelayMsgReceived, rs.receiveRelayMsgNoError)
	rs.localNode.GetEvent("relay").Subscribe(events.EventReceiveClientSignedSigChain, rs.receiveClientSignedSigChainNoError)
	return nil
}

func (rs *RelayService) HandleMsg(packet *RelayPacket) error {
	// handle packet send to self
	if bytes.Equal(rs.localNode.GetChordAddr(), packet.DestID) {
		log.Infof(
			"Receive packet:\nSrcID: %s\nDestID: %x\nPayload Size: %d",
			packet.SrcAddr,
			packet.DestID,
			len(packet.Payload),
		)
		return nil
	}

	destPubKey := packet.SigChain.GetDestPubkey()
	mining := rs.localNode.GetSyncState() == pb.PersistFinished

	err := rs.porServer.Sign(packet.SigChain, destPubKey, mining)
	if err != nil {
		log.Error("Signing signature chain error: ", err)
		return err
	}

	_, err = packet.SigChain.ExtendElement(packet.DestID, destPubKey, false)
	if err != nil {
		return err
	}

	rs.localNode.GetEvent("relay").Notify(events.EventSendInboundPacketToClient, packet)

	return nil
}

func (rs *RelayService) receiveRelayMsg(v interface{}) error {
	if packet, ok := v.(*RelayPacket); ok {
		return rs.HandleMsg(packet)
	} else {
		return errors.New("Decode relay msg failed")
	}
}

func (rs *RelayService) receiveRelayMsgNoError(v interface{}) {
	err := rs.receiveRelayMsg(v)
	if err != nil {
		log.Error(err)
	}
}

func (rs *RelayService) receiveClientSignedSigChain(v interface{}) error {
	sigChain, ok := v.(*por.SigChain)
	if !ok {
		return errors.New("Decode client signed sigchain failed")
	}

	// TODO: only pick sigchain to sign when threshold is smaller than
	buf, err := proto.Marshal(sigChain)
	if err != nil {
		return err
	}

	txn, err := vault.MakeCommitTransaction(rs.wallet, buf)
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

func (rs *RelayService) receiveClientSignedSigChainNoError(v interface{}) {
	err := rs.receiveClientSignedSigChain(v)
	if err != nil {
		log.Error(err)
	}
}

func (rs *RelayService) SignRelayPacket(nextHop *RemoteNode, packet *RelayPacket) error {
	nextPubkey, err := nextHop.GetPubKey().EncodePoint(true)
	if err != nil {
		log.Error("Get next hop public key error: ", err)
		return err
	}
	mining := false
	if rs.localNode.GetSyncState() == pb.PersistFinished {
		mining = true
	}
	err = rs.porServer.Sign(packet.SigChain, nextPubkey, mining)
	if err != nil {
		log.Error("Signing signature chain error: ", err)
		return err
	}
	return nil
}

func (localNode *LocalNode) StartRelayer() {
	localNode.relayer.Start()
}

func (localNode *LocalNode) SendRelayPacket(srcAddr, destAddr string, payload, signature []byte, maxHoldingSeconds uint32) error {
	srcID, srcPubkey, err := address.ParseClientAddress(srcAddr)
	if err != nil {
		return err
	}

	destID, destPubkey, err := address.ParseClientAddress(destAddr)
	if err != nil {
		return err
	}

	payloadHash := sha256.Sum256(payload)
	payloadHash256, err := common.Uint256ParseFromBytes(payloadHash[:])
	if err != nil {
		return err
	}

	height := ledger.DefaultLedger.Store.GetHeaderHeight() - por.SigChainBlockHeightOffset
	if height < 0 {
		height = 0
	}
	headerHash := ledger.DefaultLedger.Store.GetHeaderHashByHeight(height)
	sigChain, err := por.GetPorServer().CreateSigChainForClient(
		uint32(len(payload)),
		&payloadHash256,
		&headerHash,
		srcID,
		srcPubkey,
		destPubkey,
		signature,
		por.ECDSA,
	)
	if err != nil {
		return err
	}

	relayPacket, err := NewRelayPacket(srcAddr, destID, payload, sigChain, maxHoldingSeconds)
	if err != nil {
		return err
	}

	relayMsg, err := NewRelayMessage(relayPacket)
	if err != nil {
		return err
	}

	buf, err := relayMsg.ToBytes()
	if err != nil {
		return err
	}

	_, err = localNode.nnet.SendBytesRelayAsync(buf, destID)
	if err != nil {
		return err
	}

	return nil
}
