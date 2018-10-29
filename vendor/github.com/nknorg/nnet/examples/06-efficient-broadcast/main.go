// This example shows how to broadcast messages efficiently using Chord spanning
// tree. Naive broadcast (push) algorithm requires each node to send and receive
// the same message O(logN) times. This can be reduced to K times but it's still
// bandwidth inefficient.

// Using spanning tree structure this can be greatly reduced such that each node
// only send the message once (on average) and receive it once, thus reducing
// the total message count from O(N*logN) to N, at the cost of less robustness
// when topology is changing or there are faulty/malicious nodes. Broadcast
// latency is still near optimal as it takes at most log_2(N) steps to reach the
// whole network. This is suitable when message to be broadcasted is large and
// bandwidth is the bottleneck.

// Message redundency increase approximately linearly with NumFingerSuccessors.
// Higher redundency uses more bandwidth but gives the more robustness.

// Run with default options: go run main.go

// Show usage: go run main.go -h

package main

import (
	"flag"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nnet"
	"github.com/nknorg/nnet/log"
	"github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay/routing"
	"github.com/nknorg/nnet/protobuf"
	"github.com/nknorg/nnet/util"
)

func create(transport string, port uint16, id []byte, numFingerSuccessors uint32) (*nnet.NNet, error) {
	conf := &nnet.Config{
		Port:                  port,
		Transport:             transport,
		BaseStabilizeInterval: 233 * time.Millisecond,
		NumFingerSuccessors:   numFingerSuccessors,
	}

	nn, err := nnet.NewNNet(id, conf)
	if err != nil {
		return nil, err
	}

	return nn, nil
}

func main() {
	transportPtr := flag.String("t", "tcp", "transport type, tcp or kcp")
	numNodesPtr := flag.Int("n", 10, "number of nodes")
	numFingerSuccessorsPtr := flag.Uint("k", 1, "number of finger successors (also tree message redundency)")
	flag.Parse()

	if *numNodesPtr < 1 {
		log.Error("Number of nodes must be greater than 0")
		return
	}

	if *numFingerSuccessorsPtr < 1 {
		log.Error("Number of finger successors must be greater than 0")
		return
	}

	const createPort uint16 = 23333
	var nn *nnet.NNet
	var id []byte
	var err error
	var pushMsgCount, treeMsgCount int
	var msgCountLock sync.Mutex

	nnets := make([]*nnet.NNet, 0)

	for i := 0; i < *numNodesPtr; i++ {
		id, err = util.RandBytes(32)
		if err != nil {
			log.Error(err)
			return
		}

		nn, err = create(*transportPtr, createPort+uint16(i), id, uint32(*numFingerSuccessorsPtr))
		if err != nil {
			log.Error(err)
			return
		}

		nn.MustApplyMiddleware(routing.RemoteMessageRouted(func(remoteMessage *node.RemoteMessage, localNode *node.LocalNode, remoteNodes []*node.RemoteNode) (*node.RemoteMessage, *node.LocalNode, []*node.RemoteNode, bool) {
			if remoteMessage.Msg.MessageType == protobuf.BYTES {
				msgBody := &protobuf.Bytes{}
				err = proto.Unmarshal(remoteMessage.Msg.Message, msgBody)
				if err != nil {
					log.Error(err)
				}

				msgCountLock.Lock()
				switch remoteMessage.Msg.RoutingType {
				case protobuf.BROADCAST_PUSH:
					pushMsgCount++
					log.Infof("Receive broadcast push message \"%s\" from %x", string(msgBody.Data), remoteMessage.Msg.SrcId)
				case protobuf.BROADCAST_TREE:
					treeMsgCount++
					log.Infof("Receive broadcast tree message \"%s\" from %x", string(msgBody.Data), remoteMessage.Msg.SrcId)
				}
				msgCountLock.Unlock()
			}
			return remoteMessage, localNode, remoteNodes, true
		}))

		nnets = append(nnets, nn)
	}

	for i := 0; i < len(nnets); i++ {
		time.Sleep(112358 * time.Microsecond)

		err = nnets[i].Start()
		if err != nil {
			log.Error(err)
			return
		}

		if i > 0 {
			err = nnets[i].Join(nnets[0].GetLocalNode().Addr)
			if err != nil {
				log.Error(err)
				return
			}
		}
	}

	time.Sleep(time.Duration(*numNodesPtr/5) * time.Second)
	for i := 3; i > 0; i-- {
		log.Infof("Sending broadcast push message in %d seconds", i)
		time.Sleep(time.Second)
	}
	_, err = nnets[0].SendBytesBroadcastAsync(
		[]byte("This message should be received by EVERYONE many times!"),
		protobuf.BROADCAST_PUSH,
	)
	if err != nil {
		log.Error(err)
		return
	}

	time.Sleep(time.Second)
	for i := 3; i > 0; i-- {
		log.Infof("Sending broadcast tree message in %d seconds", i)
		time.Sleep(time.Second)
	}
	_, err = nnets[0].SendBytesBroadcastAsync(
		[]byte("This message should be received by EVERYONE almost once!"),
		protobuf.BROADCAST_TREE,
	)
	if err != nil {
		log.Error(err)
		return
	}

	time.Sleep(time.Second)
	log.Info()
	log.Info("==========================================")
	log.Infof("Total nodes count: %d", len(nnets))
	log.Infof("Total broadcast push message count: %d", pushMsgCount)
	log.Infof("Total broadcast tree message count: %d", treeMsgCount)
	log.Info("==========================================")
	log.Info()
}
