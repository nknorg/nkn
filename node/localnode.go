package node

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

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/chain/pool"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/event"
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
	"golang.org/x/time/rate"
)

type LocalNode struct {
	*Node
	nbrNodes      // neighbor nodes
	*pool.TxnPool // transaction pool of local node
	*hashCache    // txn hash cache
	*messageHandlerStore
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
	relayMessageCount uint64    // count how many messages node has relayed since start
	startTime         time.Time // Time of localNode init
	proposalSubmitted uint32    // Count of localNode submitted proposal
}

func NewLocalNode(wallet *vault.Wallet, nn *nnet.NNet) (*LocalNode, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}

	address, err := url.Parse(nn.GetLocalNode().Node.Addr)
	if err != nil {
		return nil, err
	}

	host := address.Hostname()
	tlsDomain := &TlsDomain{DotToDash(host)}

	wssDomain, err := tlsDomain.IpToDomain(config.Parameters.HttpWssDomain)
	if err != nil {
		return nil, err
	}

	httpsDomain, err := tlsDomain.IpToDomain(config.Parameters.HttpsJsonDomain)
	if err != nil {
		return nil, err
	}

	nodeData := &pb.NodeData{
		PublicKey:          account.PublicKey,
		WebsocketPort:      uint32(config.Parameters.HttpWsPort),
		JsonRpcPort:        uint32(config.Parameters.HttpJsonPort),
		ProtocolVersion:    uint32(config.ProtocolVersion),
		TlsWebsocketDomain: wssDomain,
		TlsWebsocketPort:   uint32(config.Parameters.HttpWssPort),
		TlsJsonRpcDomain:   httpsDomain,
		TlsJsonRpcPort:     uint32(config.Parameters.HttpsJsonPort),
	}

	node, err := NewNode(nn.GetLocalNode().Node.Node, nodeData)
	if err != nil {
		return nil, err
	}

	localNode := &LocalNode{
		Node:                node,
		nbrNodes:            newNbrNodes(),
		account:             account,
		TxnPool:             pool.NewTxPool(),
		quit:                make(chan bool, 1),
		hashCache:           newHashCache(),
		requestSigChainTxn:  newRequestTxn(requestSigChainTxnWorkerPoolSize, nil),
		receiveTxnMsg:       newReceiveTxnMsg(receiveTxnMsgWorkerPoolSize, nil),
		syncHeaderLimiter:   rate.NewLimiter(rate.Limit(config.Parameters.SyncBlockHeaderRateLimit), int(config.Parameters.SyncBlockHeaderRateBurst)),
		syncBlockLimiter:    rate.NewLimiter(rate.Limit(config.Parameters.SyncBlockRateLimit), int(config.Parameters.SyncBlockRateBurst)),
		messageHandlerStore: newMessageHandlerStore(),
		nnet:                nn,
		startTime:           time.Now(),
	}

	localNode.relayer = NewRelayService(wallet, localNode)

	nn.GetLocalNode().Node.Data, err = proto.Marshal(localNode.NodeData)
	if err != nil {
		return nil, err
	}

	log.Infof("Init node ID to %v", localNode.GetID())

	event.Queue.Subscribe(event.BlockPersistCompleted, localNode.cleanupTransactions)

	nn.MustApplyMiddleware(nnetnode.WillConnectToNode{func(n *nnetpb.Node) (bool, bool) {
		err := localNode.shouldConnectToNode(n)
		if err != nil {
			log.Infof("stop connect to node because: %v", err)
			return false, false
		}
		return true, true
	}, 0})

	nn.MustApplyMiddleware(nnetnode.RemoteNodeReady{func(remoteNode *nnetnode.RemoteNode) bool {
		err := localNode.verifyRemoteNode(remoteNode)
		if err != nil {
			remoteNode.Stop(err)
			return false
		}
		return true
	}, 1000})

	nn.MustApplyMiddleware(chord.NeighborAdded{func(remoteNode *nnetnode.RemoteNode, index int) bool {
		err := localNode.maybeAddRemoteNode(remoteNode)
		if err != nil {
			remoteNode.Stop(err)
			return false
		}
		return true
	}, 0})

	nn.MustApplyMiddleware(chord.NeighborRemoved{func(remoteNode *nnetnode.RemoteNode) bool {
		nbr := localNode.getNbrByNNetNode(remoteNode)
		if nbr != nil {
			localNode.DelNbrNode(nbr.GetID())
		}
		return true
	}, 0})

	nn.MustApplyMiddleware(nnetnode.MessageEncoded{func(rn *nnetnode.RemoteNode, msg []byte) ([]byte, bool) {
		return localNode.encryptMessage(msg, rn), true
	}, 0})

	nn.MustApplyMiddleware(nnetnode.MessageWillDecode{func(rn *nnetnode.RemoteNode, msg []byte) ([]byte, bool) {
		decrypted, err := localNode.decryptMessage(msg, rn)
		if err != nil {
			if localNode.getNbrByNNetNode(rn) != nil {
				rn.Stop(err)
			} else {
				log.Warningf("Decrypt message from %v error: %v", rn, err)
			}
			return nil, false
		}
		return decrypted, true
	}, 0})

	nn.MustApplyMiddleware(routing.RemoteMessageRouted{localNode.remoteMessageRouted, 0})

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
	out["uptime"] = time.Since(localNode.startTime).Truncate(time.Second).Seconds()
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
	go localNode.startConnectingToRandomNeighbors()
	return nil
}

func (localNode *LocalNode) shouldConnectToNode(n *nnetpb.Node) error {
	if n.GetData() != nil {
		nodeData := &pb.NodeData{}
		err := proto.Unmarshal(n.Data, nodeData)
		if err != nil {
			return err
		}

		if nodeData.ProtocolVersion < config.MinCompatibleProtocolVersion || nodeData.ProtocolVersion > config.MaxCompatibleProtocolVersion {
			return fmt.Errorf("remote node has protocol version %d, which is not compatible with local node protocol version %d", nodeData.ProtocolVersion, config.ProtocolVersion)
		}

		id, err := chain.DefaultLedger.Store.GetID(nodeData.PublicKey)
		if err != nil || len(id) == 0 || bytes.Equal(id, crypto.Sha256ZeroHash) {
			if localNode.GetSyncState() == pb.PERSIST_FINISHED {
				return fmt.Errorf("Remote node id can not be found in local ledger: err-%v, id-%v", err, id)
			}
		} else {
			if !bytes.Equal(id, n.GetId()) {
				return fmt.Errorf("Remote node id should be %x instead of %x", id, n.GetId())
			}
		}
	}

	if address.ShouldRejectAddr(localNode.GetAddr(), n.GetAddr()) {
		return errors.New("Remote port is different from local port")
	}

	return nil
}

func (localNode *LocalNode) verifyRemoteNode(remoteNode *nnetnode.RemoteNode) error {
	if remoteNode.GetId() == nil {
		return errors.New("Remote node id is nil")
	}

	if remoteNode.GetData() == nil {
		return errors.New("Remote node data is nil")
	}

	err := localNode.shouldConnectToNode(remoteNode.Node.Node)
	if err != nil {
		return err
	}

	addr, err := url.Parse(remoteNode.GetAddr())
	if err != nil {
		return err
	}

	connHost, connPort, err := net.SplitHostPort(remoteNode.GetConn().RemoteAddr().String())
	if err != nil {
		return err
	}

	if !address.IsPrivateIP(net.ParseIP(connHost)) && addr.Hostname() != connHost {
		return fmt.Errorf("Remote node host %s is different from its connection host %s", addr.Hostname(), connHost)
	}

	if remoteNode.IsOutbound && addr.Port() != connPort {
		return fmt.Errorf("Remote node port %v is different from its connection port %v", addr.Port(), connPort)
	}

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

func (localNode *LocalNode) GetProposalSubmitted() uint32 {
	return atomic.LoadUint32(&localNode.proposalSubmitted)
}

func (localNode *LocalNode) IncrementProposalSubmitted() {
	atomic.AddUint32(&localNode.proposalSubmitted, 1)
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
	if changed && s == pb.PERSIST_FINISHED {
		localNode.verifyNeighbors()
	}
	return changed
}

func (localNode *LocalNode) verifyNeighbors() {
	for _, nbr := range localNode.GetNeighbors(nil) {
		err := localNode.verifyRemoteNode(nbr.nnetNode)
		if err != nil {
			nbr.nnetNode.Stop(err)
		}
	}
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
			return "", "", nil, nil, errors.New("Predecessor node doesn't support WSS protocol")
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
