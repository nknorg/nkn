package node

import (
	"encoding/hex"
	"errors"
	"fmt"
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
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/relay"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
	"github.com/nknorg/nnet"
	nnetnode "github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay/chord"
	"github.com/nknorg/nnet/overlay/routing"
	nnetpb "github.com/nknorg/nnet/protobuf"
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
	sync.RWMutex
	pb.NodeData
	id            uint64         // node ID
	version       uint32         // network protocol version
	height        uint32         // node latest block height
	local         *node          // local node
	txnCnt        uint64         // transmitted transaction count
	rxTxnCnt      uint64         // received transaction count
	account       *vault.Account // local node wallet account
	publicKey     *crypto.PubKey // node public key
	flightHeights []uint32       // flight height
	nnet          *nnet.NNet
	nnetNode      *nnetnode.RemoteNode
	relayer       *relay.RelayService // relay service
	syncStopHash  Uint256             // block syncing stop hash
	syncState     pb.SyncState        // block syncing state
	quit          chan bool           // block syncing channel
	nbrNodes                          // neighbor nodes
	eventQueue                        // event queue
	*pool.TxnPool                     // transaction pool of local node
	*hashCache                        // entity hash cache
}

func NewNode() *node {
	n := node{}
	return &n
}

func NewLocalNode(account *vault.Account, nn *nnet.NNet) (Noder, error) {
	var err error

	n := NewNode()
	n.version = ProtocolVersion
	Parameters := config.Parameters

	n.PublicKey, err = account.PublicKey.EncodePoint(true)
	if err != nil {
		return nil, err
	}

	n.WebsocketPort = uint32(Parameters.HttpWsPort)
	n.JsonRpcPort = uint32(Parameters.HttpJsonPort)
	n.HttpProxyPort = uint32(Parameters.HttpProxyPort)

	n.id, err = chordIDToNodeID(nn.GetLocalNode().Id)
	if err != nil {
		return nil, err
	}
	log.Infof("Init node ID to %d", n.id)

	n.local = n
	n.account = account
	n.publicKey = account.PublicKey
	n.TxnPool = pool.NewTxnPool()
	n.syncState = pb.WaitForSyncing
	n.syncStopHash = Uint256{}
	n.quit = make(chan bool, 1)
	n.eventQueue.init()
	n.hashCache = NewHashCache()

	ledger.DefaultLedger.Blockchain.BCEvents.Subscribe(events.EventBlockPersistCompleted, n.cleanupTransactions)

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
		if remoteMessage.Msg.MessageType == nnetpb.BYTES && len(remoteMessage.Msg.ReplyToId) == 0 {
			msgBody := &nnetpb.Bytes{}
			err := proto.Unmarshal(remoteMessage.Msg.Message, msgBody)
			if err != nil {
				log.Error(err)
				return nil, nil, nil, false
			}

			if localNode != nil {
				var nbr Noder = n
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

				signedMsg := &pb.SignedMessage{}
				err = proto.Unmarshal(msgBody.Data, signedMsg)
				if err != nil {
					err = message.HandleNodeMsg(nbr, msgBody.Data)
					if err != nil {
						log.Error(err)
						return nil, nil, nil, false
					}
				} else {
					reply, err := handleMessage(nbr, signedMsg, remoteMessage.Msg.RoutingType)
					if err != nil {
						log.Error(err)
						return nil, nil, nil, false
					}

					if len(reply) > 0 {
						nbr.SendBytesReply(remoteMessage.Msg.MessageId, reply)
					}
				}

				if len(remoteNodes) == 0 {
					return nil, nil, nil, false
				}

				localNode = nil
			}

			if remoteMessage.Msg.RoutingType == nnetpb.RELAY {
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

func (node *node) Start() error {
	go node.startSyncingBlock()
	return nil
}

func (node *node) AddRemoteNode(remoteNode *nnetnode.RemoteNode) error {
	var err error
	n := NewNode()
	n.local = node
	n.nnetNode = remoteNode
	n.id, err = chordIDToNodeID(remoteNode.Id)
	if err != nil {
		return err
	}

	err = proto.Unmarshal(remoteNode.Node.Data, &n.NodeData)
	if err != nil {
		return err
	}

	n.publicKey, err = crypto.DecodePoint(n.PublicKey)
	if err != nil {
		return err
	}

	node.AddNbrNode(n)

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
	if node.IsLocalNode() {
		return ledger.DefaultLedger.Store.GetHeight()
	}

	node.RLock()
	defer node.RUnlock()
	return node.height
}

func (node *node) SetHeight(height uint32) {
	node.Lock()
	defer node.Unlock()
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
	if node.IsLocalNode() {
		return node.nnet.GetLocalNode().Addr
	}
	return node.nnetNode.Addr
}

func (node *node) GetConnDirection() string {
	if node.nnet != nil {
		return "LocalNode"
	}
	if node.nnetNode.IsOutbound { // is RemoteNode
		return "Outbound"
	}
	return "Inbound"
}

func (node *node) GetTime() int64 {
	t := time.Now()
	return t.UnixNano()
}

func (node *node) IsLocalNode() bool {
	return node.nnet != nil
}

func (node *node) SendRelayPacketsInBuffer(clientID []byte) error {
	defer func() {
		if err := recover(); err != nil {
			log.Error("SendRelayPacketsInBuffer recover:", err)
		}
	}()
	return node.relayer.SendRelayPacketsInBuffer(clientID)
}

func (node *node) GetSyncState() pb.SyncState {
	node.RLock()
	defer node.RUnlock()
	return node.syncState
}

func (node *node) SetSyncState(s pb.SyncState) {
	node.Lock()
	defer node.Unlock()
	node.syncState = s

	if node.IsLocalNode() {
		if s == pb.PersistFinished || s == pb.WaitForSyncing {
			node.syncStopHash = EmptyUint256
		}
		log.Infof("Set sync state to %s", s.String())
	}
}

func (node *node) GetSyncStopHash() Uint256 {
	node.RLock()
	defer node.RUnlock()
	return node.syncStopHash
}

func (node *node) SetSyncStopHash(hash Uint256, height uint32) {
	node.Lock()
	defer node.Unlock()
	if node.syncStopHash == EmptyUint256 && hash != EmptyUint256 {
		log.Infof("block syncing will stop when receive block: %s, height: %d",
			BytesToHexString(hash.ToArrayReverse()), height)
		node.syncStopHash = hash
	}
}

func (node *node) GetWsAddr() string {
	return fmt.Sprintf("%s:%d", node.GetAddr(), node.GetWsPort())
}

func (node *node) FindSuccessorAddrs(key []byte, numSucc int) ([]string, error) {
	if !node.IsLocalNode() {
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

func (node *node) findAddr(key []byte, portSupplier func(nodeData *protobuf.NodeData) uint32) (string, error) {
	if !node.IsLocalNode() {
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
	nodeData := &pb.NodeData{}
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

	wsAddr := fmt.Sprintf("%s:%d", host, portSupplier(nodeData))

	return wsAddr, nil
}

func (node *node) FindWsAddr(key []byte) (string, error) {
	return node.findAddr(key, func(nodeData *protobuf.NodeData) uint32 {
		return nodeData.WebsocketPort
	})
}

func (node *node) FindHttpProxyAddr(key []byte) (string, error) {
	return node.findAddr(key, func(nodeData *protobuf.NodeData) uint32 {
		return nodeData.HttpProxyPort
	})
}

func (node *node) GetChordAddr() []byte {
	if node.IsLocalNode() {
		return node.nnet.GetLocalNode().Id
	}
	return node.nnetNode.Id
}

func (node *node) CloseConn() {
	node.nnetNode.Stop(nil)
}

func (node *node) Tx(buf []byte) {
	node.SendBytesAsync(buf)
}

func (node *node) Broadcast(buf []byte) error {
	if !node.IsLocalNode() {
		return errors.New("Node is not local node")
	}

	_, err := node.nnet.SendBytesBroadcastAsync(buf, nnetpb.BROADCAST_PUSH)
	if err != nil {
		return err
	}

	return nil
}

func (node *node) DumpChordInfo() *ChordInfo {
	c, ok := node.nnet.Network.(*chord.Chord)
	if !ok {
		log.Errorf("Overlay is not chord")
		return nil
	}

	n := c.GetLocalNode()
	ret := ChordInfo{
		Node:         ChordNodeInfo{ID: hex.EncodeToString(n.GetId()), Addr: n.GetAddr()},
		Successors:   node.GetSuccessors(),
		Predecessors: node.GetPredecessors(),
		FingerTable:  node.GetFingerTab(),
	}
	ret.Node.NodeData.Unmarshal(n.GetData())
	return &ret
}

func (node *node) GetSuccessors() (ret []*ChordNodeInfo) {
	c, ok := node.nnet.Network.(*chord.Chord)
	if !ok {
		log.Errorf("Overlay is not chord")
		return []*ChordNodeInfo{}
	}
	for _, n := range c.Successors() {
		info := ChordNodeInfo{ID: hex.EncodeToString(n.GetId()), Addr: n.GetAddr(), IsOutbound: n.IsOutbound}
		info.NodeData.Unmarshal(n.GetData())
		ret = append(ret, &info)
	}
	return ret
}

func (node *node) GetPredecessors() (ret []*ChordNodeInfo) {
	c, ok := node.nnet.Network.(*chord.Chord)
	if !ok {
		log.Errorf("Overlay is not chord")
		return []*ChordNodeInfo{}
	}
	for _, n := range c.Predecessors() {
		info := ChordNodeInfo{ID: hex.EncodeToString(n.GetId()), Addr: n.GetAddr(), IsOutbound: n.IsOutbound}
		info.NodeData.Unmarshal(n.GetData())
		ret = append(ret, &info)
	}
	return ret
}

func (node *node) GetFingerTab() (ret map[int][]*ChordNodeInfo) {
	c, ok := node.nnet.Network.(*chord.Chord)
	if !ok {
		log.Errorf("Overlay is not chord")
		return make(map[int][]*ChordNodeInfo)
	}
	ret = make(map[int][]*ChordNodeInfo)
	for i, lst := range c.FingerTable() {
		if len(lst) == 0 {
			continue
		}
		ret[i] = []*ChordNodeInfo{}
		for _, n := range lst {
			info := ChordNodeInfo{ID: hex.EncodeToString(n.GetId()), Addr: n.GetAddr(), IsOutbound: n.IsOutbound}
			info.NodeData.Unmarshal(n.GetData())
			ret[i] = append(ret[i], &info)
		}
	}
	return ret
}

func (node *node) cleanupTransactions(v interface{}) {
	if block, ok := v.(*ledger.Block); ok {
		node.TxnPool.CleanSubmittedTransactions(block.Transactions)
	}
}
