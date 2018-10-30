// This example shows how to send and receive arbitrary byte messages.

// Run with default options: go run main.go

// Show usage: go run main.go -h

package main

import (
	"flag"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nnet"
	"github.com/nknorg/nnet/log"
	"github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay/chord"
	"github.com/nknorg/nnet/protobuf"
	"github.com/nknorg/nnet/util"
)

func create(transport string, port uint16, id []byte) (*nnet.NNet, error) {
	conf := &nnet.Config{
		Port:                  port,
		Transport:             transport,
		BaseStabilizeInterval: 233 * time.Millisecond,
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
	flag.Parse()

	if *numNodesPtr < 1 {
		log.Error("Number of nodes must be greater than 0")
		return
	}

	const createPort uint16 = 23333
	var nn *nnet.NNet
	var id []byte
	var err error

	nnets := make([]*nnet.NNet, 0)

	for i := 0; i < *numNodesPtr; i++ {
		id, err = util.RandBytes(32)
		if err != nil {
			log.Error(err)
			return
		}

		nn, err = create(*transportPtr, createPort+uint16(i), id)
		if err != nil {
			log.Error(err)
			return
		}

		nn.MustApplyMiddleware(node.BytesReceived(func(msg, msgID, srcID []byte, remoteNode *node.RemoteNode) ([]byte, bool) {
			log.Infof("Receive message \"%s\" from %x by %x", string(msg), srcID, remoteNode.Id)

			_, err = nn.SendBytesRelayReply(msgID, []byte("Well received!"), srcID)
			if err != nil {
				log.Error(err)
			}

			return msg, true
		}))

		nnets = append(nnets, nn)
	}

	nnets[0].MustApplyMiddleware(chord.FingerTableAdded(func(remoteNode *node.RemoteNode, fingerIndex, nodeIndex int) bool {
		err = nnets[0].SendBytesDirectAsync([]byte("Hello my finger!"), remoteNode)
		if err != nil {
			log.Error(err)
		}
		return true
	}))

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
		log.Infof("Sending broadcast message in %d seconds", i)
		time.Sleep(time.Second)
	}
	_, err = nnets[0].SendBytesBroadcastAsync(
		[]byte("This message should be received by EVERYONE!"),
		protobuf.BROADCAST_PUSH,
	)
	if err != nil {
		log.Error(err)
		return
	}

	time.Sleep(time.Second)
	for i := 3; i > 0; i-- {
		log.Infof("Sending relay message in %d seconds", i)
		time.Sleep(time.Second)
	}
	reply, _, err := nnets[0].SendBytesRelaySync([]byte("This message should only be received by SOMEONE!"), id)
	if err != nil {
		log.Error(err)
		return
	}
	msgBody := &protobuf.Bytes{}
	err = proto.Unmarshal(reply.Message, msgBody)
	if err != nil {
		log.Error(err)
	}
	log.Infof("Receive reply message \"%s\" from %x", string(msgBody.Data), reply.SrcId)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	log.Info("\nReceived an interrupt, stopping...\n")

	var wg sync.WaitGroup
	for i := 0; i < len(nnets); i++ {
		wg.Add(1)
		go func(nn *nnet.NNet) {
			nn.Stop(nil)
			wg.Done()
		}(nnets[len(nnets)-1-i])
	}
	wg.Wait()
}
