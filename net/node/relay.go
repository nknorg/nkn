package node

import (
	"crypto/sha256"
	"errors"
	"net"
	"strconv"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/relay"
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
	iter, err := chordNode.ClosestNeighborIterator(key)
	if err != nil {
		return nil, err
	}
	for {
		nbr := iter.Next()
		if nbr == nil {
			break
		}
		nbrAddr, err := nbr.NodeAddr()
		if err != nil {
			continue
		}
		found := false
		var n protocol.Noder
		var ip net.IP
		for _, tn := range node.nbrNodes.List {
			addr := getNodeAddr(tn)
			ip = addr.IpAddr[:]
			addrstring := ip.To16().String() + ":" + strconv.Itoa(int(addr.Port))
			if nbrAddr == addrstring {
				n = tn
				found = true
				break
			}
		}
		if found {
			if n.GetState() == protocol.ESTABLISH {
				return n, nil
			}
		}
	}
	return nil, nil
}

func (node *node) SendRelayPacket(srcID, srcPubkey, destID, destPubkey, payload, signature []byte) error {
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
		por.SigAlgo_ECDSA,
	)

	relayPacket, err := message.NewRelayPacket(srcID, destID, payload, sigChain)
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
