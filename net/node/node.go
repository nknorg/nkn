package node

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/pool"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/chord"
	. "github.com/nknorg/nkn/net/message"
	. "github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/relay"
	. "github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
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
	state                    uint32              // node connection state
	id                       uint64              // node ID
	cap                      [32]byte            // node capability
	version                  uint32              // network protocol version
	services                 uint64              // supplied services
	relay                    bool                // relay capability (merge into capbility flag)
	height                   uint32              // node latest block height
	local                    *node               // local node
	txnCnt                   uint64              // transmitted transaction count
	rxTxnCnt                 uint64              // received transaction count
	publicKey                *crypto.PubKey      // node public key
	flightHeights            []uint32            // flight height
	headerReqChan            ChanQueue           // semaphore for connection counts
	msgHandlerChan           ChanQueue           // chan for message handler
	chordAddr                []byte              // chord address (chord ID)
	ring                     *chord.Ring         // chord ring
	relayer                  *relay.RelayService // relay service
	syncStopHash             Uint256             // block syncing stop hash
	syncState                SyncState           // block syncing state
	quit                     chan bool           // block syncing channel
	nodeDisconnectSubscriber events.Subscriber   // disconnect event
	link                                         // link status and information
	nbrNodes                                     // neighbor nodes
	eventQueue                                   // event queue
	*pool.TxnPool                                // transaction pool of local node
	*hashCache                                   // entity hash cache
	ConnectingNodes                              // connecting nodes cache
	RetryConnAddrs                               // retry connection cache
}

type RetryConnAddrs struct {
	sync.RWMutex
	RetryAddrs map[string]int
}

type ConnectingNodes struct {
	sync.RWMutex
	ConnectingAddrs []string
}

func (node *node) DumpInfo() {
	log.Info("Node info:")
	log.Info("\t state = ", node.state)
	log.Info("\t syncState = ", node.syncState)
	log.Info(fmt.Sprintf("\t id = 0x%x", node.id))
	log.Info("\t addr = ", node.addr)
	log.Info("\t cap = ", node.cap)
	log.Info("\t version = ", node.version)
	log.Info("\t services = ", node.services)
	log.Info("\t port = ", node.port)
	log.Info("\t relay = ", node.relay)
	log.Info("\t height = ", node.height)
	log.Info("\t conn cnt = ", node.link.connCnt)
}

func (node *node) SetAddrInConnectingList(addr string) (added bool) {
	node.ConnectingNodes.Lock()
	defer node.ConnectingNodes.Unlock()
	for _, a := range node.ConnectingAddrs {
		if a == addr {
			return false
		}
	}
	node.ConnectingAddrs = append(node.ConnectingAddrs, addr)
	go func() {
		time.Sleep(ConnectingTimeout)
		node.RemoveAddrInConnectingList(addr)
	}()
	return true
}

func (node *node) RemoveAddrInConnectingList(addr string) {
	node.ConnectingNodes.Lock()
	defer node.ConnectingNodes.Unlock()
	for i, a := range node.ConnectingAddrs {
		if a == addr {
			node.ConnectingAddrs = append(node.ConnectingAddrs[:i], node.ConnectingAddrs[i+1:]...)
			return
		}
	}
}

func (node *node) UpdateInfo(t time.Time, version uint32, services uint64,
	port uint16, nonce uint64, relay uint8, height uint32) {

	node.UpdateRXTime(t)
	node.id = nonce
	node.version = version
	node.services = services
	node.port = port
	if relay == 0 {
		node.relay = false
	} else {
		node.relay = true
	}
	node.height = height
}

func NewNode() *node {
	n := node{
		state: INIT,
	}
	n.time = time.Now()
	go func() {
		time.Sleep(ConnectingTimeout)
		if n.local != &n && n.GetState() != ESTABLISH {
			log.Warn("Connecting timeout:", n.GetAddrStr(), "with state", n.GetState())
			n.SetState(INACTIVITY)
			n.CloseConn()
		}
	}()
	return &n
}

func InitNode(pubKey *crypto.PubKey, ring *chord.Ring) Noder {
	n := NewNode()
	n.version = ProtocolVersion
	if Parameters.MaxHdrSyncReqs <= 0 {
		n.headerReqChan = MakeChanQueue(MaxSyncHeaderReq)
	} else {
		n.headerReqChan = MakeChanQueue(Parameters.MaxHdrSyncReqs)
	}
	n.link.addr = Parameters.Hostname
	n.link.port = Parameters.NodePort
	n.link.chordPort = Parameters.ChordPort
	n.link.webSockPort = Parameters.HttpWsPort
	n.link.httpJSONPort = Parameters.HttpJsonPort
	n.relay = true

	chordVnode, err := ring.GetFirstVnode()
	if err != nil {
		log.Error(err)
	}
	n.chordAddr = chordVnode.Id

	err = binary.Read(bytes.NewBuffer(n.chordAddr), binary.LittleEndian, &(n.id))
	if err != nil {
		log.Error(err)
	}
	log.Info(fmt.Sprintf("Init node ID to 0x%x", n.id))
	n.nbrNodes.init()
	n.local = n
	n.publicKey = pubKey
	n.TxnPool = pool.NewTxnPool()
	n.syncState = SyncStarted
	n.syncStopHash = Uint256{}
	n.msgHandlerChan = MakeChanQueue(MaxMsgChanNum)
	n.quit = make(chan bool, 1)
	n.eventQueue.init()
	n.hashCache = NewHashCache()
	n.nodeDisconnectSubscriber = n.eventQueue.GetEvent("disconnect").Subscribe(events.EventNodeDisconnect, n.NodeDisconnected)
	n.ring = ring

	go n.initConnection()
	go n.updateConnection()
	go n.keepalive()

	return n
}

func (n *node) NodeDisconnected(v interface{}) {
	if node, ok := v.(*node); ok {
		node.SetState(INACTIVITY)
		node.CloseConn()
	}
}

func (node *node) GetID() uint64 {
	return node.id
}

func (node *node) GetState() uint32 {
	return atomic.LoadUint32(&(node.state))
}

func (node *node) GetConn() net.Conn {
	return node.conn
}

func (node *node) GetPort() uint16 {
	return node.port
}

func (node *node) GetHttpJsonPort() uint16 {
	return node.httpJSONPort
}

func (node *node) GetWebSockPort() uint16 {
	return node.webSockPort
}

func (node *node) GetChordPort() uint16 {
	return node.chordPort
}

func (node *node) GetHttpInfoPort() uint16 {
	return node.httpInfoPort
}

func (node *node) SetHttpInfoPort(nodeInfoPort uint16) {
	node.httpInfoPort = nodeInfoPort
}

func (node *node) GetHttpInfoState() bool {
	if node.cap[HTTPINFOFLAG] == 0x01 {
		return true
	} else {
		return false
	}
}

func (node *node) SetHttpInfoState(nodeInfo bool) {
	if nodeInfo {
		node.cap[HTTPINFOFLAG] = 0x01
	} else {
		node.cap[HTTPINFOFLAG] = 0x00
	}
}

func (node *node) GetRelay() bool {
	return node.relay
}

func (node *node) Version() uint32 {
	return node.version
}

func (node *node) Services() uint64 {
	return node.services
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

func (node *node) SetState(state uint32) {
	atomic.StoreUint32(&(node.state), state)
	if state == ESTABLISH || state == INACTIVITY {
		node.LocalNode().RemoveAddrInConnectingList(node.GetAddrStr())
	}
}

func (node *node) GetPubKey() *crypto.PubKey {
	return node.publicKey
}

func (node *node) CompareAndSetState(old, new uint32) bool {
	return atomic.CompareAndSwapUint32(&(node.state), old, new)
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

func (node *node) UpdateRXTime(t time.Time) {
	node.time = t
}

func (node *node) Xmit(message interface{}) error {
	var buffer []byte
	var err error
	switch message.(type) {
	case *transaction.Transaction:
		txn := message.(*transaction.Transaction)
		buffer, err = NewTxn(txn)
		if err != nil {
			log.Error("Error New Tx message: ", err)
			return err
		}
		node.txnCnt++
		node.ExistHash(txn.Hash())
	case *ledger.Block:
		block := message.(*ledger.Block)
		buffer, err = NewBlock(block)
		if err != nil {
			log.Error("Error New Block message: ", err)
			return err
		}
	case *ConsensusPayload:
		consensusPayload := message.(*ConsensusPayload)
		buffer, err = NewConsensus(consensusPayload)
		if err != nil {
			log.Error("Error New consensus message: ", err)
			return err
		}
	case *IsingPayload:
		isingPayload := message.(*IsingPayload)
		buffer, err = NewIsingConsensus(isingPayload)
		if err != nil {
			log.Error("Error New ising consensus message: ", err)
			return err
		}
	case Uint256:
		hash := message.(Uint256)
		buf := bytes.NewBuffer([]byte{})
		hash.Serialize(buf)
		// construct inv message
		invPayload := NewInvPayload(BLOCK, 1, buf.Bytes())
		buffer, err = NewInv(invPayload)
		if err != nil {
			log.Error("Error New inv message")
			return err
		}
	default:
		log.Warn("Unknown Xmit message type")
		return errors.New("Unknown Xmit message type")
	}

	node.nbrNodes.Broadcast(buffer)

	return nil
}

func (node *node) BroadcastTransaction(from Noder, txn *transaction.Transaction) error {
	buffer, err := NewTxn(txn)
	if err != nil {
		log.Error("Error New Tx message: ", err)
		return err
	}
	node.txnCnt++
	for _, n := range node.GetNeighborNoder() {
		if n.GetRelay() && n.GetID() != from.GetID() {
			n.Tx(buffer)
		}
	}

	return nil
}

func (node *node) GetAddr() string {
	return node.addr
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
	return net.JoinHostPort(node.GetAddr(), strconv.Itoa(int(node.GetPort())))
}

func (node *node) GetTime() int64 {
	t := time.Now()
	return t.UnixNano()
}

func (node *node) GetBookKeeperAddr() *crypto.PubKey {
	return node.publicKey
}

func (node *node) GetBookKeepersAddrs() ([]*crypto.PubKey, uint64) {
	pks := make([]*crypto.PubKey, 1)
	pks[0] = node.publicKey
	var i uint64
	i = 1
	for _, n := range node.GetNeighborNoder() {
		pktmp := n.GetBookKeeperAddr()
		pks = append(pks, pktmp)
		i++
	}
	return pks, i
}

func (node *node) SetBookKeeperAddr(pk *crypto.PubKey) {
	node.publicKey = pk
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

func (node *node) GetLastRXTime() time.Time {
	return node.time
}

func (node *node) AddInRetryList(addr string) {
	node.RetryConnAddrs.Lock()
	defer node.RetryConnAddrs.Unlock()
	if node.RetryAddrs == nil {
		node.RetryAddrs = make(map[string]int)
	}
	if _, ok := node.RetryAddrs[addr]; ok {
		delete(node.RetryAddrs, addr)
		log.Debug("remove addr from retry list", addr)
	}
	//alway set retry to 0
	node.RetryAddrs[addr] = 0
	log.Debug("add addr to retry list", addr)
}

func (node *node) RemoveFromRetryList(addr string) {
	node.RetryConnAddrs.Lock()
	defer node.RetryConnAddrs.Unlock()
	if len(node.RetryAddrs) > 0 {
		if _, ok := node.RetryAddrs[addr]; ok {
			delete(node.RetryAddrs, addr)
			log.Debug("remove addr from retry list", addr)
		}
	}

}
func (node *node) AcquireMsgHandlerChan() {
	node.msgHandlerChan.acquire()
}

func (node *node) ReleaseMsgHandlerChan() {
	node.msgHandlerChan.release()
}

func (node *node) AcquireHeaderReqChan() {
	node.headerReqChan.acquire()
}

func (node *node) ReleaseHeaderReqChan() {
	node.headerReqChan.release()
}

func (node *node) GetChordAddr() []byte {
	return node.chordAddr
}

func (node *node) SetChordAddr(addr []byte) {
	node.chordAddr = addr
}

func (node *node) GetChordRing() *chord.Ring {
	return node.ring
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
	SendMsgSyncHeaders(n, stopHash)
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
				if ledger.DefaultLedger.Store.BlockInCache(hash) == false {
					ReqBlkData(n, hash)
				}
			}

		}
		for i = 1; i <= count && dValue >= 0; i++ {
			hash := ledger.DefaultLedger.Store.GetHeaderHashByHeight(currentBlkHeight + reqCnt)

			if ledger.DefaultLedger.Store.BlockInCache(hash) == false {
				ReqBlkData(n, hash)
				n.StoreFlightHeight(currentBlkHeight + reqCnt)
			}
			reqCnt++
			dValue--
		}
	}
}

func (node *node) SendPingToNbr() {
	noders := node.local.GetNeighborNoder()
	for _, n := range noders {
		if n.GetState() == ESTABLISH {
			buf, err := NewPingMsg(node.syncState)
			if err != nil {
				log.Error("failed build a new ping message")
			} else {
				go n.Tx(buf)
			}
		}
	}
}

func (node *node) HeartBeatMonitor() {
	neighbors := node.local.GetActiveNeighbors()
	for _, n := range neighbors {
		t := n.GetLastRXTime()
		if time.Now().Sub(t) > KeepaliveTimeout {
			log.Warn("Keepalive timeout:", n.GetAddrStr())
			n.SetState(INACTIVITY)
			n.CloseConn()
		}
	}
}

func (node *node) ConnectNeighbors() {
	chordNode, err := node.ring.GetFirstVnode()
	if err != nil || chordNode == nil {
		return
	}
	neighbors := chordNode.GetNeighbors()
	for _, chordNbr := range neighbors {
		if !node.IsChordAddrInNeighbors(chordNbr.Id) {
			chordNbrAddr, err := chordNbr.NodeAddr()
			if err != nil {
				continue
			}
			log.Infof("Trying to connect to chord node %x at %s", chordNbr.Id, chordNbrAddr)
			go node.Connect(chordNbrAddr)
		}
	}
}

func (node *node) DisconnectNeighbor(nbr *node) {
	_, success := node.nbrNodes.DelNbrNode(nbr.GetID())
	if success {
		nbr.SetState(INACTIVITY)
		nbr.CloseConn()
	}
}

func (n *node) DisconnectNonNeighbors() {
	for _, nodeNbr := range n.GetNeighborNoder() {
		nodeNbr.GetConnectionCnt()
		_, port, err := net.SplitHostPort(nodeNbr.GetConn().LocalAddr().String())
		if err != nil {
			log.Error(err)
			continue
		}
		// Skip inbound connections
		if port == strconv.Itoa(int(Parameters.NodePort)) {
			continue
		}
		shouldInNbr, err := n.ShouldChordAddrInNeighbors(nodeNbr.GetChordAddr())
		if err != nil {
			log.Error(err)
			continue
		}
		if !shouldInNbr {
			log.Info("Disconnect non chord neighbor:", nodeNbr.GetAddrStr())
			nbr, ok := nodeNbr.(*node)
			if ok {
				go n.DisconnectNeighbor(nbr)
			}
		}
	}
}

func getNodeAddr(n *node) NodeAddr {
	var addr NodeAddr
	addr.IpAddr, _ = n.GetAddr16()
	addr.Time = n.GetTime()
	addr.Services = n.Services()
	addr.Port = n.GetPort()
	addr.ID = n.GetID()
	return addr
}

func (node *node) keepalive() {
	ticker := time.NewTicker(KeepAliveTicker)
	for {
		select {
		case <-ticker.C:
			node.SendPingToNbr()
			node.HeartBeatMonitor()
		}
	}
}

func (node *node) updateConnection() {
	t := time.NewTicker(ConnectionTicker)
	for {
		select {
		case <-t.C:
			node.ConnectNeighbors()
			node.DisconnectNonNeighbors()
			// node.TryConnect()
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
