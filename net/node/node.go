package node

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/pool"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/message"
	. "github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/protobuf"
	"github.com/nknorg/nkn/relay"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nnet"
	nnetnode "github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay/chord"
	"github.com/nknorg/nnet/overlay/routing"
	nnetprotobuf "github.com/nknorg/nnet/protobuf"
)

const (
	MaxSyncHeaderReq     = 2                // max concurrent sync header request count
	MaxMsgChanNum        = 2048             // max goroutine num for message handler
	ConnectionMaxBackoff = 4000             // back off for retry
	MaxRetryCount        = 3                // max retry count
	KeepAliveTicker      = 3 * time.Second  // ticker for ping/pong and keepalive message
	KeepaliveTimeout     = 9 * time.Second  // timeout for keeping alive
	BlockSyncingTicker   = 3 * time.Second  // ticker for syncing block
	ProtocolVersion      = 0                // protocol version
	ConnectionTicker     = 10 * time.Second // ticker for connection
	MaxReqBlkOnce        = 16               // max block count requested
	ConnectingTimeout    = 10 * time.Second // timeout for waiting for connection
)

type node struct {
	sync.Mutex
	protobuf.NodeData
	id            uint64         // node ID
	version       uint32         // network protocol version
	height        uint32         // node latest block height
	local         *node          // local node
	txnCnt        uint64         // transmitted transaction count
	rxTxnCnt      uint64         // received transaction count
	publicKey     *crypto.PubKey // node public key
	flightHeights []uint32       // flight height
	nnet          *nnet.NNet
	nnetNode      *nnetnode.RemoteNode
	relayer       *relay.RelayService // relay service
	syncStopHash  Uint256             // block syncing stop hash
	syncState     SyncState           // block syncing state
	quit          chan bool           // block syncing channel
	nbrNodes                          // neighbor nodes
	eventQueue                        // event queue
	*pool.TxnPool                     // transaction pool of local node
	*hashCache                        // entity hash cache
}

// Generates an ID for the node
func GenChordID(host string) []byte {
	hash := sha256.New()
	hash.Write([]byte(host))
	return hash.Sum(nil)
}

func chordIDToNodeID(chordID []byte) (uint64, error) {
	var nodeID uint64
	err := binary.Read(bytes.NewBuffer(chordID), binary.LittleEndian, &nodeID)
	return nodeID, err
}

func (n *node) getNbrByNNetNode(remoteNode *nnetnode.RemoteNode) *node {
	if remoteNode == nil {
		return nil
	}

	nodeID, err := chordIDToNodeID(remoteNode.Id)
	if err != nil {
		return nil
	}

	nbr := n.GetNbrNode(nodeID)
	return nbr
}

func (node *node) DumpInfo() {
	log.Info("Node info:")
	log.Info("\t syncState = ", node.syncState)
	log.Info(fmt.Sprintf("\t id = 0x%x", node.id))
	log.Info("\t addr = ", node.GetAddr())
	log.Info("\t version = ", node.version)
	log.Info("\t port = ", node.GetPort())
	log.Info("\t height = ", node.height)
}

func NewNode() *node {
	n := node{}
	return &n
}

func InitNode(pubKey *crypto.PubKey, nn *nnet.NNet) (Noder, error) {
	var err error

	n := NewNode()
	n.version = ProtocolVersion
	Parameters := config.Parameters

	n.PublicKey, err = pubKey.EncodePoint(true)
	if err != nil {
		return nil, err
	}

	n.WebsocketPort = uint32(Parameters.HttpWsPort)
	n.JsonRpcPort = uint32(Parameters.HttpJsonPort)

	n.id, err = chordIDToNodeID(nn.GetLocalNode().Id)
	if err != nil {
		return nil, err
	}
	log.Infof("Init node ID to %d", n.id)

	n.local = n
	n.publicKey = pubKey
	n.TxnPool = pool.NewTxnPool()
	n.syncState = SyncStarted
	n.syncStopHash = Uint256{}
	n.quit = make(chan bool, 1)
	n.eventQueue.init()
	n.hashCache = NewHashCache()
	n.nnet = nn

	nn.GetLocalNode().Node.Data, err = proto.Marshal(&n.NodeData)
	if err != nil {
		return nil, err
	}

	nn.MustApplyMiddleware(nnetnode.RemoteNodeReady(func(remoteNode *nnetnode.RemoteNode) bool {
		if address.ShouldRejectAddr(n.GetAddrStr(), remoteNode.Addr) {
			remoteNode.Stop(errors.New("Remote port is different from local port"))
			return false
		}
		return true
	}))

	nn.MustApplyMiddleware(chord.NeighborAdded(func(remoteNode *nnetnode.RemoteNode, index int) bool {
		err := n.AddRemoteNode(remoteNode)
		if err != nil {
			remoteNode.Stop(err)
			return false
		}
		return true
	}))

	nn.MustApplyMiddleware(chord.NeighborRemoved(func(remoteNode *nnetnode.RemoteNode) bool {
		nbr := n.getNbrByNNetNode(remoteNode)
		if nbr != nil {
			n.DelNbrNode(nbr.GetID())
		}

		return true
	}))

	nn.MustApplyMiddleware(routing.RemoteMessageRouted(func(remoteMessage *nnetnode.RemoteMessage, localNode *nnetnode.LocalNode, remoteNodes []*nnetnode.RemoteNode) (*nnetnode.RemoteMessage, *nnetnode.LocalNode, []*nnetnode.RemoteNode, bool) {
		if remoteMessage.Msg.MessageType == nnetprotobuf.BYTES {
			msgBody := &nnetprotobuf.Bytes{}
			err := proto.Unmarshal(remoteMessage.Msg.Message, msgBody)
			if err != nil {
				log.Error(err)
				return nil, nil, nil, false
			}

			if localNode != nil {
				nbr := n
				if remoteMessage.RemoteNode != nil {
					nbr = n.getNbrByNNetNode(remoteMessage.RemoteNode)
					if nbr == nil {
						err = n.AddRemoteNode(remoteMessage.RemoteNode)
						if err != nil {
							log.Error("Cannot add remote node:", err)
							return nil, nil, nil, false
						}

						nbr = n.getNbrByNNetNode(remoteMessage.RemoteNode)
						if nbr == nil {
							log.Error("Cannot get neighbor node")
							return nil, nil, nil, false
						}
					}
				}

				err = message.HandleNodeMsg(nbr, msgBody.Data)
				if err != nil {
					log.Error(err)
					return nil, nil, nil, false
				}

				if len(remoteNodes) == 0 {
					return nil, nil, nil, false
				}

				localNode = nil
			}

			if remoteMessage.Msg.RoutingType == nnetprotobuf.RELAY {
				if len(remoteNodes) > 1 {
					log.Error("Multiple next hop is not supported yet")
					return nil, nil, nil, false
				}

				nextHop := n.getNbrByNNetNode(remoteNodes[0])
				if nextHop == nil {
					err := n.AddRemoteNode(remoteNodes[0])
					if err != nil {
						log.Error("Cannot add next hop remote node:", err)
						return nil, nil, nil, false
					}

					nextHop = n.getNbrByNNetNode(remoteNodes[0])
					if nextHop == nil {
						log.Error("Cannot get next hop neighbor node")
						return nil, nil, nil, false
					}
				}

				msgBody := &nnetprotobuf.Bytes{}
				err := proto.Unmarshal(remoteMessage.Msg.Message, msgBody)
				if err != nil {
					log.Error(err)
					return nil, nil, nil, false
				}

				msg, err := message.ParseMsg(msgBody.Data)
				if err != nil {
					log.Error(err)
					return nil, nil, nil, false
				}

				relayMsg, ok := msg.(*message.RelayMessage)
				if !ok {
					log.Error("Msg is not relay message")
					return nil, nil, nil, false
				}

				relayPacket := &relayMsg.Packet
				err = n.relayer.SignRelayPacket(nextHop, relayPacket)
				if err != nil {
					log.Error(err)
					return nil, nil, nil, false
				}

				relayMsg, err = message.NewRelayMessage(relayPacket)
				if err != nil {
					log.Error(err)
					return nil, nil, nil, false
				}

				msgBody.Data, err = relayMsg.ToBytes()
				if err != nil {
					log.Error(err)
					return nil, nil, nil, false
				}

				remoteMessage.Msg.Message, err = proto.Marshal(msgBody)
				if err != nil {
					log.Error(err)
					return nil, nil, nil, false
				}
			}
		}

		return remoteMessage, localNode, remoteNodes, true
	}))

	return n, nil
}

func (n *node) AddRemoteNode(remoteNode *nnetnode.RemoteNode) error {
	var err error
	node := NewNode()
	node.local = n
	node.nnetNode = remoteNode
	node.id, err = chordIDToNodeID(remoteNode.Id)
	if err != nil {
		return err
	}

	err = proto.Unmarshal(remoteNode.Node.Data, &node.NodeData)
	if err != nil {
		return err
	}

	node.publicKey, err = crypto.DecodePoint(node.PublicKey)
	if err != nil {
		return err
	}

	n.AddNbrNode(node)

	return nil
}

func (node *node) GetID() uint64 {
	return node.id
}

func (node *node) GetPort() uint16 {
	address, _ := url.Parse(node.GetAddrStr())
	port, _ := strconv.Atoi(address.Port())
	return uint16(port)
}

func (node *node) GetHttpJsonPort() uint16 {
	return uint16(node.JsonRpcPort)
}

func (node *node) GetWsPort() uint16 {
	return uint16(node.WebsocketPort)
}

func (node *node) Version() uint32 {
	return node.version
}

func (node *node) IncRxTxnCnt() {
	node.rxTxnCnt++
}

func (node *node) GetTxnCnt() uint64 {
	return node.txnCnt
}

func (node *node) GetRxTxnCnt() uint64 {
	return node.rxTxnCnt
}

func (node *node) GetPubKey() *crypto.PubKey {
	return node.publicKey
}

func (node *node) LocalNode() Noder {
	return node.local
}

func (node *node) GetTxnPool() *pool.TxnPool {
	return node.TxnPool
}

func (node *node) GetHeight() uint32 {
	return node.height
}

func (node *node) SetHeight(height uint32) {
	//TODO read/write lock
	node.height = height
}

func (node *node) Xmit(msg interface{}) error {
	var buffer []byte
	var err error
	switch msg.(type) {
	case *transaction.Transaction:
		txn := msg.(*transaction.Transaction)
		buffer, err = message.NewTxn(txn)
		if err != nil {
			log.Error("Error New Tx message: ", err)
			return err
		}
		node.txnCnt++
		node.ExistHash(txn.Hash())
	default:
		log.Warning("Unknown Xmit message type")
		return errors.New("Unknown Xmit message type")
	}

	return node.Broadcast(buffer)
}

func (node *node) GetAddr() string {
	address, _ := url.Parse(node.GetAddrStr())
	return address.Hostname()
}

func (node *node) GetAddr16() ([16]byte, error) {
	var result [16]byte
	ip := net.ParseIP(node.GetAddr()).To16()
	if ip == nil {
		log.Error("Parse IP address error\n")
		return result, errors.New("Parse IP address error")
	}

	copy(result[:], ip[:16])
	return result, nil
}

func (node *node) GetAddrStr() string {
	if node.nnet != nil {
		return node.nnet.GetLocalNode().Addr
	}
	return node.nnetNode.Addr
}

func (node *node) GetTime() int64 {
	t := time.Now()
	return t.UnixNano()
}

func (node *node) WaitForSyncHeaderFinish(isProposer bool) {
	if isProposer {
		for {
			//TODO: proposer node syncs block from 50% neighbors
			heights, _ := node.GetNeighborHeights()
			if CompareHeight(ledger.DefaultLedger.Blockchain.BlockHeight, heights) {
				break
			}
			<-time.After(time.Second)
		}
	} else {
		for {
			if node.syncStopHash != EmptyUint256 {
				// return if the stop hash has been saved
				header, err := ledger.DefaultLedger.Blockchain.GetHeader(node.syncStopHash)
				if err == nil && header != nil {
					break
				}
			}
			<-time.After(time.Second)
		}
	}
}

func (node *node) WaitForSyncBlkFinish() {
	for {
		headerHeight := ledger.DefaultLedger.Store.GetHeaderHeight()
		currentBlkHeight := ledger.DefaultLedger.Blockchain.BlockHeight
		log.Debug("WaitForSyncBlkFinish... current block height is ", currentBlkHeight, " ,current header height is ", headerHeight)
		if currentBlkHeight >= headerHeight {
			break
		}
		<-time.After(2 * time.Second)
	}
}

func (node *node) StoreFlightHeight(height uint32) {
	node.flightHeights = append(node.flightHeights, height)
}

func (node *node) GetFlightHeightCnt() int {
	return len(node.flightHeights)
}

func (node *node) GetFlightHeights() []uint32 {
	return node.flightHeights
}

func (node *node) RemoveFlightHeightLessThan(h uint32) {
	heights := node.flightHeights
	p := len(heights)
	i := 0

	for i < p {
		if heights[i] < h {
			p--
			heights[p], heights[i] = heights[i], heights[p]
		} else {
			i++
		}
	}
	node.flightHeights = heights[:p]
}

func (node *node) RemoveFlightHeight(height uint32) {
	node.flightHeights = SliceRemove(node.flightHeights, height)
}

func (node *node) blockHeaderSyncing(stopHash Uint256) {
	noders := node.local.GetNeighborNoder()
	if len(noders) == 0 {
		return
	}
	nodelist := []Noder{}
	for _, v := range noders {
		if ledger.DefaultLedger.Store.GetHeaderHeight() < v.GetHeight() {
			nodelist = append(nodelist, v)
		}
	}
	ncout := len(nodelist)
	if ncout == 0 {
		return
	}
	index := rand.Intn(ncout)
	n := nodelist[index]
	message.SendMsgSyncHeaders(n, stopHash)
}

func (node *node) blockSyncing() {
	headerHeight := ledger.DefaultLedger.Store.GetHeaderHeight()
	currentBlkHeight := ledger.DefaultLedger.Blockchain.BlockHeight
	if currentBlkHeight >= headerHeight {
		return
	}
	var dValue int32
	var reqCnt uint32
	var i uint32
	noders := node.local.GetNeighborNoder()

	for _, n := range noders {
		if uint32(n.GetHeight()) <= currentBlkHeight {
			continue
		}
		n.RemoveFlightHeightLessThan(currentBlkHeight)
		count := MaxReqBlkOnce - uint32(n.GetFlightHeightCnt())
		dValue = int32(headerHeight - currentBlkHeight - reqCnt)
		flights := n.GetFlightHeights()
		if count == 0 {
			for _, f := range flights {
				hash := ledger.DefaultLedger.Store.GetHeaderHashByHeight(f)
				if !ledger.DefaultLedger.Store.BlockInCache(hash) {
					message.ReqBlkData(n, hash)
				}
			}

		}
		for i = 1; i <= count && dValue >= 0; i++ {
			hash := ledger.DefaultLedger.Store.GetHeaderHashByHeight(currentBlkHeight + reqCnt)

			if !ledger.DefaultLedger.Store.BlockInCache(hash) {
				message.ReqBlkData(n, hash)
				n.StoreFlightHeight(currentBlkHeight + reqCnt)
			}
			reqCnt++
			dValue--
		}
	}
}

func (node *node) SyncBlock(isProposer bool) {
	ticker := time.NewTicker(BlockSyncingTicker)
	for {
		select {
		case <-ticker.C:
			if isProposer {
				node.blockHeaderSyncing(EmptyUint256)
			} else if node.syncStopHash != EmptyUint256 {
				node.blockHeaderSyncing(node.syncStopHash)
			}
			node.blockSyncing()
		case skip := <-node.quit:
			log.Info("block syncing finished")
			ticker.Stop()
			node.LocalNode().GetEvent("sync").Notify(events.EventBlockSyncingFinished, skip)
			return
		}
	}
}

func (node *node) StopSyncBlock(skip bool) {
	// switch syncing state
	node.SetSyncState(SyncFinished)
	// stop block syncing
	node.quit <- skip
}

func (node *node) SyncBlockMonitor(isProposer bool) {
	// wait for header syncing finished
	node.WaitForSyncHeaderFinish(isProposer)
	// wait for block syncing finished
	node.WaitForSyncBlkFinish()
	// stop block syncing
	node.StopSyncBlock(false)
}

func (node *node) SendRelayPacketsInBuffer(clientId []byte) error {
	defer func() {
		if err := recover(); err != nil {
			log.Error("SendRelayPacketsInBuffer recover:", err)
		}
	}()
	return node.relayer.SendRelayPacketsInBuffer(clientId)
}

func (node *node) GetSyncState() SyncState {
	return node.syncState
}

func (node *node) SetSyncState(s SyncState) {
	node.syncState = s
}

func (node *node) SetSyncStopHash(hash Uint256, height uint32) {
	node.Lock()
	defer node.Unlock()
	if node.syncStopHash == EmptyUint256 {
		log.Infof("block syncing will stop when receive block: %s, height: %d",
			BytesToHexString(hash.ToArrayReverse()), height)
		node.syncStopHash = hash
	}
}

func (node *node) GetWsAddr() string {
	return fmt.Sprintf("%s:%d", node.GetAddr(), node.GetWsPort())
}

func (node *node) FindSuccessorAddrs(key []byte, numSucc int) ([]string, error) {
	if node.nnet == nil {
		return nil, errors.New("Node is not local node")
	}

	c, ok := node.nnet.Network.(*chord.Chord)
	if !ok {
		return nil, errors.New("Overlay is not chord")
	}

	succs, err := c.FindSuccessors(key, uint32(numSucc))
	if err != nil {
		return nil, err
	}

	if len(succs) == 0 {
		return nil, errors.New("Found no successors")
	}

	addrs := make([]string, len(succs))
	for i := range succs {
		addrs[i] = succs[i].Addr
	}

	return addrs, nil
}

func (node *node) FindWsAddr(key []byte) (string, error) {
	if node.nnet == nil {
		return "", errors.New("Node is not local node")
	}

	c, ok := node.nnet.Network.(*chord.Chord)
	if !ok {
		return "", errors.New("Overlay is not chord")
	}

	preds, err := c.FindPredecessors(key, 1)
	if err != nil {
		return "", err
	}

	if len(preds) == 0 {
		return "", errors.New("Found no predecessors")
	}

	pred := preds[0]
	nodeData := &protobuf.NodeData{}
	err = proto.Unmarshal(pred.Data, nodeData)
	if err != nil {
		return "", err
	}

	address, _ := url.Parse(pred.Addr)
	if err != nil {
		return "", err
	}

	host := address.Hostname()
	if host == "" {
		return "", errors.New("Hostname is empty")
	}

	wsAddr := fmt.Sprintf("%s:%d", host, nodeData.WebsocketPort)

	return wsAddr, nil
}

func (node *node) GetChordAddr() []byte {
	if node.nnet != nil {
		return node.nnet.GetLocalNode().Id
	}
	return node.nnetNode.Id
}

func (node *node) CloseConn() {
	node.nnetNode.Stop(nil)
}

func (node *node) Tx(buf []byte) {
	err := node.local.nnet.SendBytesDirectAsync(buf, node.nnetNode)
	if err != nil {
		log.Error("Error sending messge to peer node ", err.Error())
		node.CloseConn()
		node.local.DelNbrNode(node.GetID())
	}
}

func (node *node) Broadcast(buf []byte) error {
	if node.nnet == nil {
		return errors.New("Node is not local node")
	}

	_, err := node.nnet.SendBytesBroadcastAsync(buf, nnetprotobuf.BROADCAST_PUSH)
	if err != nil {
		return err
	}

	return nil
}
