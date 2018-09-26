package node

import (
	"crypto/sha256"
	"errors"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/relay"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
)

func (node *node) StartRelayer(wallet vault.Wallet) {
	node.relayer = relay.NewRelayService(wallet, node)
	node.relayer.Start()
}

func (node *node) NextHop(key []byte) (protocol.Noder, error) {
	chordNode, err := node.ring.GetFirstVnode()
	if err != nil || chordNode == nil {
		return nil, errors.New("No chord node binded")
	}

	smallestDist := node.ring.Distance(chordNode.Id, key)
	var nextHop protocol.Noder
	neighbors := node.GetNeighborNoder()
	for _, neighbor := range neighbors {
		dist := node.ring.Distance(neighbor.GetChordAddr(), key)
		if dist.Cmp(smallestDist) < 0 {
			smallestDist = dist
			nextHop = neighbor
		}
	}

	return nextHop, nil

	// iter, err := chordNode.ClosestNeighborIterator(key)
	// if err != nil {
	// 	return nil, err
	// }
	// for {
	// 	chordNbr := iter.Next()
	// 	if chordNbr == nil {
	// 		break
	// 	}
	// 	nodeNbr := node.GetNeighborByChordAddr(chordNbr.Id)
	// 	if nodeNbr != nil {
	// 		return nodeNbr, nil
	// 	}
	// }
	// return nil, nil
}

func (node *node) SendRelayPacket(srcAddr, destAddr string, payload, signature []byte) error {
	srcID, srcPubkey, err := address.ParseClientAddress(srcAddr)
	if err != nil {
		log.Error("Parse src client address error:", err)
		return err
	}

	destID, destPubkey, err := address.ParseClientAddress(destAddr)
	if err != nil {
		log.Error("Parse dest client address error:", err)
		return err
	}

	payloadHash := sha256.Sum256(payload)
	payloadHash256, err := common.Uint256ParseFromBytes(payloadHash[:])
	if err != nil {
		log.Error("Compute uint256 data hash error: ", err)
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

	relayPacket, err := message.NewRelayPacket(srcAddr, destID, payload, sigChain)
	if err != nil {
		log.Error("Create relay packet error: ", err)
		return err
	}

	err = node.relayer.HandleMsg(relayPacket)
	if err != nil {
		log.Error("Handle relay msg error:", err)
		return err
	}

	return nil
}
