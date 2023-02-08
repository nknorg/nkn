package lnode

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/api/ratelimiter"
	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/chain/pool"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/event"
	"github.com/nknorg/nkn/v2/node"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/util/log"
	"github.com/nknorg/nkn/v2/vault"
	"github.com/nknorg/nnet"
	nnetnode "github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay/chord"
	"github.com/nknorg/nnet/overlay/routing"
	nnetpb "github.com/nknorg/nnet/protobuf"
	"golang.org/x/time/rate"
)

type LocalNode struct {
	*node.Node
	*neighborNodes // neighbor nodes
	*pool.TxnPool  // transaction pool of local node
	*hashCache     // txn hash cache
	*node.MessageHandlerStore
	account            *vault.Account // local node wallet account
	nnet               *nnet.NNet     // nnet instance
	relayer            *RelayService  // relay service
	quit               chan bool      // block syncing channel
	requestSigChainTxn *requestTxn
	receiveTxnMsg      *receiveTxnMsg
	syncHeaderLimiter  *rate.Limiter
	syncBlockLimiter   *rate.Limiter

	mu                sync.RWMutex
	syncOnce          *sync.Once
	relayMessageCount uint64 // count how many messages node has relayed since start
	proposalSubmitted uint32 // Count of localNode submitted proposal
}

func NewLocalNode(wallet *vault.Wallet, nn *nnet.NNet) (*LocalNode, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}

	var ledgerMode pb.LedgerMode
	if chain.DefaultLedger.Store.GetHeight() > 0 {
		if _, err = chain.DefaultLedger.Store.GetBlockByHeight(1); err != nil {
			ledgerMode = pb.LedgerMode_light
		} else {
			ledgerMode = pb.LedgerMode_full
		}
	} else {
		if config.Parameters.IsLightSync() {
			ledgerMode = pb.LedgerMode_light
		} else {
			ledgerMode = pb.LedgerMode_full
		}
	}

	nodeData := &pb.NodeData{
		PublicKey:          account.PublicKey,
		WebsocketPort:      uint32(config.Parameters.HttpWsPort),
		JsonRpcPort:        uint32(config.Parameters.HttpJsonPort),
		ProtocolVersion:    uint32(config.ProtocolVersion),
		TlsWebsocketDomain: config.Parameters.HttpWssDomain,
		TlsWebsocketPort:   uint32(config.Parameters.HttpWssPort),
		TlsJsonRpcDomain:   config.Parameters.HttpsJsonDomain,
		TlsJsonRpcPort:     uint32(config.Parameters.HttpsJsonPort),
		LedgerMode:         ledgerMode,
	}

	n, err := node.NewNode(nn.GetLocalNode().Node.Node, nodeData)
	if err != nil {
		return nil, err
	}

	localNode := &LocalNode{
		Node:                n,
		neighborNodes:       newNeighborNodes(),
		account:             account,
		TxnPool:             pool.NewTxPool(),
		quit:                make(chan bool, 1),
		hashCache:           newHashCache(),
		requestSigChainTxn:  newRequestTxn(requestSigChainTxnWorkerPoolSize, nil),
		receiveTxnMsg:       newReceiveTxnMsg(receiveTxnMsgWorkerPoolSize, nil),
		syncHeaderLimiter:   rate.NewLimiter(rate.Limit(config.Parameters.SyncBlockHeaderRateLimit), int(config.Parameters.SyncBlockHeaderRateBurst)),
		syncBlockLimiter:    rate.NewLimiter(rate.Limit(config.Parameters.SyncBlockRateLimit), int(config.Parameters.SyncBlockRateBurst)),
		MessageHandlerStore: node.NewMessageHandlerStore(),
		nnet:                nn,
	}

	localNode.relayer = NewRelayService(wallet, localNode)

	nn.GetLocalNode().Node.Data, err = proto.Marshal(localNode.NodeData)
	if err != nil {
		return nil, err
	}

	log.Infof("Init node ID to %v", localNode.GetID())

	event.Queue.Subscribe(event.BlockPersistCompleted, localNode.cleanupTransactions)
	event.Queue.Subscribe(event.NewBlockProduced, localNode.CheckIDChange)

	nn.MustApplyMiddleware(nnetnode.ConnectionAccepted{Func: func(conn net.Conn) (bool, bool) {
		host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
		if err == nil {
			limiter := ratelimiter.GetLimiter("node:"+host, config.Parameters.NodeIPRateLimit, int(config.Parameters.NodeIPRateBurst))
			if !limiter.Allow() {
				log.Infof("Node connection limit of %s reached", host)
				return false, false
			}
		}
		return true, true
	}})

	nn.MustApplyMiddleware(nnetnode.WillConnectToNode{Func: func(n *nnetpb.Node) (bool, bool) {
		err := localNode.shouldConnectToNode(n)
		if err != nil {
			log.Infof("stop connect to node because: %v", err)
			return false, false
		}
		return true, true
	}})

	nn.MustApplyMiddleware(nnetnode.RemoteNodeReady{Func: func(remoteNode *nnetnode.RemoteNode) bool {
		err := localNode.verifyRemoteNode(remoteNode)
		if err != nil {
			remoteNode.Stop(err)
			return false
		}
		return true
	}, Priority: 1000})

	var startOnce sync.Once
	nn.MustApplyMiddleware(chord.NeighborAdded{Func: func(remoteNode *nnetnode.RemoteNode, index int) bool {
		startOnce.Do(localNode.startConnectingToRandomNeighbors)
		err := localNode.maybeAddRemoteNode(remoteNode)
		if err != nil {
			remoteNode.Stop(err)
			return false
		}
		return true
	}})

	nn.MustApplyMiddleware(chord.NeighborRemoved{Func: func(remoteNode *nnetnode.RemoteNode) bool {
		nbr := localNode.GetNeighborByNNetNode(remoteNode)
		if nbr != nil {
			localNode.RemoveNeighborNode(nbr.GetID())
		}
		return true
	}})

	nn.MustApplyMiddleware(nnetnode.MessageEncoded{Func: func(rn *nnetnode.RemoteNode, msg []byte) ([]byte, bool) {
		return localNode.encryptMessage(msg, rn), true
	}})

	nn.MustApplyMiddleware(nnetnode.MessageWillDecode{Func: func(rn *nnetnode.RemoteNode, msg []byte) ([]byte, bool) {
		decrypted, err := localNode.decryptMessage(msg, rn)
		if err != nil {
			if localNode.GetNeighborByNNetNode(rn) != nil {
				rn.Stop(err)
			} else {
				log.Warningf("Decrypt message from %v error: %v", rn, err)
			}
			return nil, false
		}
		return decrypted, true
	}})

	nn.MustApplyMiddleware(routing.RemoteMessageRouted{Func: localNode.remoteMessageRouted})

	nn.MustApplyMiddleware(chord.RelayPriority{Func: func(rn *nnetnode.RemoteNode, priority float64) (float64, bool) {
		var connTime time.Duration
		nbr := localNode.GetNeighborByNNetNode(rn)
		if nbr != nil {
			connTime = time.Since(nbr.Node.StartTime)
		}
		priority = relayPriority(priority, connTime)
		return priority, true
	}})

	return localNode, nil
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
	out["uptime"] = time.Since(localNode.Node.StartTime) / time.Second
	out["version"] = config.Version
	out["relayMessageCount"] = localNode.GetRelayMessageCount()
	if config.Parameters.MiningDebug {
		out["proposalSubmitted"] = localNode.GetProposalSubmitted()
		out["currTimeStamp"] = time.Now().Unix()
	}

	return json.Marshal(out)
}

func (localNode *LocalNode) Start() error {
	localNode.startRelayer()
	localNode.initSyncing()
	localNode.initTxnHandlers()
	return nil
}

func (localNode *LocalNode) GetProposalSubmitted() uint32 {
	return atomic.LoadUint32(&localNode.proposalSubmitted)
}

func (localNode *LocalNode) IncrementProposalSubmitted() {
	atomic.AddUint32(&localNode.proposalSubmitted, 1)
}

func (localNode *LocalNode) GetNnet() *nnet.NNet {
	return localNode.nnet
}

func (localNode *LocalNode) GetRelayMessageCount() uint64 {
	localNode.mu.RLock()
	defer localNode.mu.RUnlock()
	return localNode.relayMessageCount
}

func (localNode *LocalNode) IncrementRelayMessageCount() {
	localNode.mu.Lock()
	localNode.relayMessageCount++
	localNode.mu.Unlock()
}

func (localNode *LocalNode) GetTxnPool() *pool.TxnPool {
	return localNode.TxnPool
}

func (localNode *LocalNode) GetHeight() uint32 {
	return chain.DefaultLedger.Store.GetHeight()
}

func (localNode *LocalNode) SetSyncState(s pb.SyncState) bool {
	log.Infof("Set sync state to %s", s.String())
	changed := localNode.Node.SetSyncState(s)
	if changed && s == pb.SyncState_PERSIST_FINISHED {
		config.SyncPruning = config.LivePruning
		localNode.verifyNeighbors()
	}
	return changed
}

func (localNode *LocalNode) GetSyncState() pb.SyncState {
	return localNode.Node.GetSyncState()
}

func (localNode *LocalNode) SetMinVerifiableHeight(height uint32) {
	log.Infof("Set min verifiable height to %d", height)
	localNode.Node.SetMinVerifiableHeight(height)
}

func (localNode *LocalNode) GetWsAddr() string {
	return fmt.Sprintf("%s:%d", localNode.GetHostname(), localNode.GetWebsocketPort())
}

func (localNode *LocalNode) GetWssAddr() string {
	return fmt.Sprintf("%s:%d", localNode.GetTlsWebsocketDomain(), localNode.GetTlsWebsocketPort())
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

func (localNode *LocalNode) findAddrForClient(key []byte, tls bool) (string, string, []byte, []byte, error) {
	c, ok := localNode.nnet.Network.(*chord.Chord)
	if !ok {
		return "", "", nil, nil, errors.New("Overlay is not chord")
	}

	preds, err := c.FindPredecessors(key, 1)
	if err != nil {
		return "", "", nil, nil, err
	}
	if len(preds) == 0 {
		return "", "", nil, nil, errors.New("Found no predecessors")
	}

	pred := preds[0]
	nodeData := &pb.NodeData{}
	err = proto.Unmarshal(pred.Data, nodeData)
	if err != nil {
		return "", "", nil, nil, err
	}

	var wsHost, rpcHost string
	var wsPort, rpcPort uint16

	if tls == true {
		// We only check wss because https rpc is optional
		if len(nodeData.TlsWebsocketDomain) == 0 || nodeData.TlsWebsocketPort == 0 {
			return "", "", nil, nil, errors.New("predecessor node doesn't support WSS protocol")
		}
		wsHost = nodeData.TlsWebsocketDomain
		wsPort = uint16(nodeData.TlsWebsocketPort)
		rpcHost = nodeData.TlsJsonRpcDomain
		rpcPort = uint16(nodeData.TlsJsonRpcPort)
	} else {
		address, err := url.Parse(pred.Addr)
		if err != nil {
			return "", "", nil, nil, err
		}
		wsHost = address.Hostname()
		if wsHost == "" {
			return "", "", nil, nil, errors.New("Hostname is empty")
		}
		wsPort = uint16(nodeData.WebsocketPort)
		rpcHost = wsHost
		rpcPort = uint16(nodeData.JsonRpcPort)
	}

	wsAddr := fmt.Sprintf("%s:%d", wsHost, wsPort)
	rpcAddr := fmt.Sprintf("%s:%d", rpcHost, rpcPort)

	return wsAddr, rpcAddr, nodeData.PublicKey, pred.Id, nil
}

func (localNode *LocalNode) FindWsAddr(key []byte) (string, string, []byte, []byte, error) {
	return localNode.findAddrForClient(key, false)
}

func (localNode *LocalNode) FindWssAddr(key []byte) (string, string, []byte, []byte, error) {
	return localNode.findAddrForClient(key, true)
}

func (localNode *LocalNode) CheckIDChange(v interface{}) {
	localHeight := chain.DefaultLedger.Store.GetHeight()
	id, err := chain.DefaultLedger.Store.GetID(localNode.PublicKey, localHeight)
	if err != nil {
		log.Fatalf("local node has no id:%v", err)
	}
	if !bytes.Equal(localNode.Id, id) {
		log.Fatalf("local node id has changed")
	}
}
