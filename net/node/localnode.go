package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction/pool"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
	"github.com/nknorg/nnet"
	nnetnode "github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay/chord"
	"github.com/nknorg/nnet/overlay/routing"
)

const (
	MaxSyncHeaderReq     = 2                // max concurrent sync header request count
	MaxMsgChanNum        = 2048             // max goroutine num for message handler
	ConnectionMaxBackoff = 4000             // back off for retry
	MaxRetryCount        = 3                // max retry count
	KeepAliveTicker      = 3 * time.Second  // ticker for ping/pong and keepalive message
	KeepaliveTimeout     = 9 * time.Second  // timeout for keeping alive
	BlockSyncingTicker   = 3 * time.Second  // ticker for syncing block
	ConnectionTicker     = 10 * time.Second // ticker for connection
	MaxReqBlkOnce        = 16               // max block count requested
	ConnectingTimeout    = 10 * time.Second // timeout for waiting for connection
)

type LocalNode struct {
	*Node
	account       *vault.Account // local node wallet account
	nnet          *nnet.NNet     // nnet instance
	relayer       *RelayService  // relay service
	quit          chan bool      // block syncing channel
	nbrNodes                     // neighbor nodes
	eventQueue                   // event queue
	*pool.TxnPool                // transaction pool of local node
	*hashCache                   // entity hash cache
	*messageHandlerStore

	sync.RWMutex
	syncOnce *sync.Once
}

func (localNode *LocalNode) MarshalJSON() ([]byte, error) {
	var out map[string]interface{}

	buf, err := json.Marshal(localNode.Node)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(buf, &out)
	if err != nil {
		return nil, err
	}

	out["height"] = localNode.GetHeight()
	out["version"] = config.Version

	return json.Marshal(out)
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

	nodeData := &pb.NodeData{
		PublicKey:     publicKey,
		WebsocketPort: uint32(config.Parameters.HttpWsPort),
		JsonRpcPort:   uint32(config.Parameters.HttpJsonPort),
		HttpProxyPort: uint32(config.Parameters.HttpProxyPort),
	}

	node, err := NewNode(nn.GetLocalNode().Node.Node, nodeData)
	if err != nil {
		return nil, err
	}

	localNode := &LocalNode{
		Node:                node,
		account:             account,
		TxnPool:             pool.NewTxnPool(),
		quit:                make(chan bool, 1),
		hashCache:           NewHashCache(),
		messageHandlerStore: newMessageHandlerStore(),
		nnet:                nn,
	}

	localNode.relayer = NewRelayService(wallet, localNode)

	nn.GetLocalNode().Node.Data, err = proto.Marshal(localNode.NodeData)
	if err != nil {
		return nil, err
	}

	log.Infof("Init node ID to %v", localNode.GetID())

	localNode.eventQueue.init()

	ledger.DefaultLedger.Blockchain.BCEvents.Subscribe(events.EventBlockPersistCompleted, localNode.cleanupTransactions)

	nn.MustApplyMiddleware(nnetnode.RemoteNodeReady(func(remoteNode *nnetnode.RemoteNode) bool {
		if address.ShouldRejectAddr(localNode.GetAddr(), remoteNode.GetAddr()) {
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
	localNode.startRelayer()
	localNode.initSyncing()
	localNode.AddMessageHandler(pb.TRANSACTIONS, localNode.transactionsMessageHandler)
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

func (localNode *LocalNode) GetTxnPool() *pool.TxnPool {
	return localNode.TxnPool
}

func (localNode *LocalNode) GetHeight() uint32 {
	return ledger.DefaultLedger.Store.GetHeight()
}

func (localNode *LocalNode) SetSyncState(s pb.SyncState) {
	localNode.Node.Lock()
	defer localNode.Node.Unlock()
	localNode.syncState = s
	log.Infof("Set sync state to %s", s.String())
}

func (localNode *LocalNode) GetWsAddr() string {
	return fmt.Sprintf("%s:%d", localNode.GetHostname(), localNode.GetWebsocketPort())
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
