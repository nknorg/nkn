package node

import (
	"errors"
	"net"
	"strconv"

	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/relay"
)

func (node *node) StartRelayer() {
	node.relayer = relay.NewRelayService(node)
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

func (node *node) SendRelayPacket(v interface{}) error {
	err := node.relayer.ReceiveRelayMsg(v)
	return err
}
