package node

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/relay"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
)

func (node *node) StartRelayer(account *wallet.Account) {
	node.relayer = relay.NewRelayService(account, node)
	node.relayer.Start()
}

func (node *node) NextHop(key []byte) (protocol.Noder, error) {
	chordNode := node.ring.GetFirstVnode()
	if chordNode == nil {
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
	nextHop, err := node.NextHop(destID)
	if err != nil {
		log.Error("Get next hop error: ", err)
		return err
	}
	if nextHop == nil {
		return errors.New(fmt.Sprintf("No next hop for destination %x", destID))
	}
	nextPubkey, err := nextHop.GetPubKey().EncodePoint(true)
	if err != nil {
		log.Error("Get next hop public key error: ", err)
		return err
	}
	payloadHash := sha256.Sum256(payload)
	payloadHash256, err := common.Uint256ParseFromBytes(payloadHash[:])
	if err != nil {
		log.Error("Compute uint256 data hash error: ", err)
		return err
	}
	sigChain, err := por.GetPorServer().CreateSigChainForClient(
		ledger.DefaultLedger.Store.GetHeight(),
		uint32(len(payload)),
		&payloadHash256,
		srcPubkey,
		destPubkey,
		nextPubkey,
		signature,
	)
	if err != nil {
		log.Error("Create signature chain for client error: ", err)
		return err
	}
	relayPacket, err := message.NewRelayPacket(srcID, destID, payload, sigChain)
	if err != nil {
		log.Error("Create relay packet error: ", err)
		return err
	}
	msg, err := message.NewRelayMessage(relayPacket)
	if err != nil {
		log.Error("Create relay message error: ", err)
		return err
	}
	msgBytes, err := msg.ToBytes()
	if err != nil {
		log.Error("Convert relay message to bytes error: ", err)
		return err
	}
	nextHop.Tx(msgBytes)
	return nil
}
