package node

import (
	"crypto/sha256"
	"errors"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/relay"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/vault"
)

func (node *node) StartRelayer(wallet vault.Wallet) {
	node.relayer = relay.NewRelayService(wallet, node)
	node.relayer.Start()
}

func (node *node) SendRelayPacket(srcAddr, destAddr string, payload, signature []byte, maxHoldingSeconds uint32) error {
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

	height := ledger.DefaultLedger.Store.GetHeaderHeight()
	prevHeaderHash := ledger.DefaultLedger.Store.GetHeaderHashByHeight(height - 1)
	sigChain, err := por.GetPorServer().CreateSigChainForClient(
		uint32(len(payload)),
		&payloadHash256,
		&prevHeaderHash,
		srcID,
		srcPubkey,
		destPubkey,
		signature,
		por.ECDSA,
	)
	if err != nil {
		return err
	}

	relayPacket, err := message.NewRelayPacket(srcAddr, destID, payload, sigChain, maxHoldingSeconds)
	if err != nil {
		return err
	}

	relayMsg, err := message.NewRelayMessage(relayPacket)
	if err != nil {
		return err
	}

	buf, err := relayMsg.ToBytes()
	if err != nil {
		return err
	}

	if node.nnet == nil {
		return errors.New("Node is not local node")
	}
	_, err = node.nnet.SendBytesRelayAsync(buf, destID)
	if err != nil {
		return err
	}

	return nil
}
