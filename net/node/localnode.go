package node

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/pool"
	"github.com/nknorg/nkn/events"
	. "github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/pb"
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

type LocalNode struct {
	*Node
	sync.RWMutex
	version       uint32         // network protocol version
	txnCnt        uint64         // transmitted transaction count
	rxTxnCnt      uint64         // received transaction count
	account       *vault.Account // local node wallet account
	nnet          *nnet.NNet     // nnet instance
	relayer       *RelayService  // relay service
	syncStopHash  Uint256        // block syncing stop hash
	quit          chan bool      // block syncing channel
	nbrNodes                     // neighbor nodes
	eventQueue                   // event queue
	*pool.TxnPool                // transaction pool of local node
	*hashCache                   // entity hash cache
	*messageHandlerStore
}

func NewLocalNode(wallet vault.Wallet, nn *nnet.NNet) (*LocalNode, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}

	publicKey, err := account.PublicKey.EncodePoint(true)
	if err != nil {
		return nil, err
	}

	nodeData := pb.NodeData{
		PublicKey:     publicKey,
		WebsocketPort: uint32(config.Parameters.HttpWsPort),
		JsonRpcPort:   uint32(config.Parameters.HttpJsonPort),
		HttpProxyPort: uint32(config.Parameters.HttpProxyPort),
	}

	node, err := NewNode(nn.GetLocalNode().Id, account.PublicKey, nodeData)
	if err != nil {
		return nil, err
	}

	localNode := &LocalNode{
		Node:                node,
		version:             ProtocolVersion,
		account:             account,
		TxnPool:             pool.NewTxnPool(),
		syncStopHash:        Uint256{},
		quit:                make(chan bool, 1),
		hashCache:           NewHashCache(),
		messageHandlerStore: newMessageHandlerStore(),
		nnet:                nn,
	}

	localNode.relayer = NewRelayService(wallet, localNode)

	nn.GetLocalNode().Node.Data, err = proto.Marshal(&localNode.NodeData)
	if err != nil {
		return nil, err
	}

	log.Infof("Init node ID to %d", localNode.id)

	localNode.eventQueue.init()

	ledger.DefaultLedger.Blockchain.BCEvents.Subscribe(events.EventBlockPersistCompleted, localNode.cleanupTransactions)

	nn.MustApplyMiddleware(nnetnode.RemoteNodeReady(func(remoteNode *nnetnode.RemoteNode) bool {
		if address.ShouldRejectAddr(localNode.GetAddrStr(), remoteNode.Addr) {
			remoteNode.Stop(errors.New("Remote port is different from local port"))
			return false
		}
		return true
	}))

	nn.MustApplyMiddleware(chord.NeighborAdded(func(remoteNode *nnetnode.RemoteNode, index int) bool {
		err := localNode.maybeAddRemoteNode(remoteNode)
		if err != nil {
			remoteNode.Stop(err)
			return false
		}
		return true
	}))

	nn.MustApplyMiddleware(chord.NeighborRemoved(func(remoteNode *nnetnode.RemoteNode) bool {
		nbr := localNode.getNbrByNNetNode(remoteNode)
		if nbr != nil {
			localNode.DelNbrNode(nbr.GetID())
		}

		return true
	}))

	nn.MustApplyMiddleware(routing.RemoteMessageRouted(localNode.remoteMessageRouted))

	return localNode, nil
}

func (localNode *LocalNode) Start() error {
	localNode.StartRelayer()
	go localNode.startSyncingBlock()
	return nil
}

func (localNode *LocalNode) addRemoteNode(nnetNode *nnetnode.RemoteNode) error {
	remoteNode, err := NewRemoteNode(localNode, nnetNode)
	if err != nil {
		return err
	}

	err = localNode.AddNbrNode(remoteNode)
	if err != nil {
		return err
	}

	return nil
}

func (localNode *LocalNode) maybeAddRemoteNode(remoteNode *nnetnode.RemoteNode) error {
	if remoteNode != nil && localNode.getNbrByNNetNode(remoteNode) == nil {
		return localNode.addRemoteNode(remoteNode)
	}
	return nil
}

func (localNode *LocalNode) Version() uint32 {
	return localNode.version
}

func (localNode *LocalNode) IncRxTxnCnt() {
	localNode.rxTxnCnt++
}

func (localNode *LocalNode) GetTxnCnt() uint64 {
	return localNode.txnCnt
}

func (localNode *LocalNode) GetRxTxnCnt() uint64 {
	return localNode.rxTxnCnt
}

func (localNode *LocalNode) GetTxnPool() *pool.TxnPool {
	return localNode.TxnPool
}

func (localNode *LocalNode) GetHeight() uint32 {
	return ledger.DefaultLedger.Store.GetHeight()
}

func (localNode *LocalNode) Xmit(msg interface{}) error {
	var buffer []byte
	var err error
	switch msg.(type) {
	case *transaction.Transaction:
		txn := msg.(*transaction.Transaction)
		buffer, err = NewTxn(txn)
		if err != nil {
			log.Error("Error New Tx message: ", err)
			return err
		}
		localNode.txnCnt++
		localNode.ExistHash(txn.Hash())
	default:
		log.Warning("Unknown Xmit message type")
		return errors.New("Unknown Xmit message type")
	}

	return localNode.Broadcast(buffer)
}

func (localNode *LocalNode) GetAddrStr() string {
	return localNode.nnet.GetLocalNode().Addr
}

func (localNode *LocalNode) GetAddr() string {
	address, _ := url.Parse(localNode.GetAddrStr())
	return address.Hostname()
}

func (localNode *LocalNode) GetPort() uint16 {
	address, _ := url.Parse(localNode.GetAddrStr())
	port, _ := strconv.Atoi(address.Port())
	return uint16(port)
}

func (localNode *LocalNode) SetSyncState(s pb.SyncState) {
	localNode.Node.Lock()
	defer localNode.Node.Unlock()
	localNode.syncState = s

	if s == pb.PersistFinished || s == pb.WaitForSyncing {
		localNode.syncStopHash = EmptyUint256
	}

	log.Infof("Set sync state to %s", s.String())
}

func (localNode *LocalNode) GetSyncStopHash() Uint256 {
	localNode.RLock()
	defer localNode.RUnlock()
	return localNode.syncStopHash
}

func (localNode *LocalNode) SetSyncStopHash(hash Uint256, height uint32) {
	localNode.Lock()
	defer localNode.Unlock()
	if localNode.syncStopHash == EmptyUint256 && hash != EmptyUint256 {
		log.Infof("block syncing will stop when receive block: %s, height: %d",
			BytesToHexString(hash.ToArrayReverse()), height)
		localNode.syncStopHash = hash
	}
}

func (localNode *LocalNode) GetWsAddr() string {
	return fmt.Sprintf("%s:%d", localNode.GetAddr(), localNode.GetWsPort())
}

func (localNode *LocalNode) FindSuccessorAddrs(key []byte, numSucc int) ([]string, error) {
	c, ok := localNode.nnet.Network.(*chord.Chord)
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

func (localNode *LocalNode) findAddr(key []byte, portSupplier func(nodeData *pb.NodeData) uint32) (string, error) {
	c, ok := localNode.nnet.Network.(*chord.Chord)
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

func (localNode *LocalNode) FindWsAddr(key []byte) (string, error) {
	return localNode.findAddr(key, func(nodeData *pb.NodeData) uint32 {
		return nodeData.WebsocketPort
	})
}

func (localNode *LocalNode) FindHttpProxyAddr(key []byte) (string, error) {
	return localNode.findAddr(key, func(nodeData *pb.NodeData) uint32 {
		return nodeData.HttpProxyPort
	})
}

func (localNode *LocalNode) GetChordAddr() []byte {
	return localNode.nnet.GetLocalNode().Id
}

func (localNode *LocalNode) Broadcast(buf []byte) error {
	_, err := localNode.nnet.SendBytesBroadcastAsync(buf, nnetpb.BROADCAST_PUSH)
	if err != nil {
		return err
	}

	return nil
}

func (localNode *LocalNode) DumpChordInfo() *ChordInfo {
	c, ok := localNode.nnet.Network.(*chord.Chord)
	if !ok {
		log.Errorf("Overlay is not chord")
		return nil
	}

	n := c.GetLocalNode()
	ret := ChordInfo{
		Node:         ChordNodeInfo{ID: hex.EncodeToString(n.GetId()), Addr: n.GetAddr()},
		Successors:   localNode.GetSuccessors(),
		Predecessors: localNode.GetPredecessors(),
		FingerTable:  localNode.GetFingerTable(),
	}
	ret.Node.NodeData.Unmarshal(n.GetData())
	return &ret
}

func (localNode *LocalNode) GetSuccessors() (ret []*ChordNodeInfo) {
	c, ok := localNode.nnet.Network.(*chord.Chord)
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

func (localNode *LocalNode) GetPredecessors() (ret []*ChordNodeInfo) {
	c, ok := localNode.nnet.Network.(*chord.Chord)
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

func (localNode *LocalNode) GetFingerTable() (ret map[int][]*ChordNodeInfo) {
	c, ok := localNode.nnet.Network.(*chord.Chord)
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

func (localNode *LocalNode) cleanupTransactions(v interface{}) {
	if block, ok := v.(*ledger.Block); ok {
		localNode.TxnPool.CleanSubmittedTransactions(block.Transactions)
	}
}
