package node

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/crypto"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/por"
	"github.com/nknorg/nkn/v2/util"
	"github.com/nknorg/nkn/v2/util/address"
	"github.com/nknorg/nkn/v2/util/log"
	nnetnode "github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay/chord"
	nnetpb "github.com/nknorg/nnet/protobuf"
)

const (
	randomNeighborsConnectDelay    = config.ConsensusTimeout / 2
	gossipNeighborsConnectInterval = 4 * config.ConsensusTimeout
	votingNeighborsConnectInterval = 1 * config.ConsensusTimeout
	removeChanSize                 = 1024
)

type randomNeighbors struct {
	connectInterval time.Duration
	removeChan      chan string

	sync.RWMutex
	ids []string
}

func newRandomNeighbors(connectInterval time.Duration) *randomNeighbors {
	return &randomNeighbors{
		connectInterval: connectInterval,
		removeChan:      make(chan string, removeChanSize),
		ids:             make([]string, 0),
	}
}

func (rn *randomNeighbors) maybeRemove(id string) {
	select {
	case rn.removeChan <- id:
	default:
	}
}

type neighborNodes struct {
	nodes           sync.Map
	gossipNeighbors *randomNeighbors
	votingNeighbors *randomNeighbors
}

func newNeighborNodes() *neighborNodes {
	return &neighborNodes{
		gossipNeighbors: newRandomNeighbors(gossipNeighborsConnectInterval),
		votingNeighbors: newRandomNeighbors(votingNeighborsConnectInterval),
	}
}

func (nm *neighborNodes) GetNeighborNode(id string) *RemoteNode {
	if v, ok := nm.nodes.Load(id); ok {
		if n, ok := v.(*RemoteNode); ok {
			return n
		}
	}
	return nil
}

func (nm *neighborNodes) addNeighborNode(remoteNode *RemoteNode) {
	nm.nodes.LoadOrStore(remoteNode.GetID(), remoteNode)
}

func (nm *neighborNodes) removeNeighborNode(id string) {
	nm.nodes.Delete(id)
	nm.gossipNeighbors.maybeRemove(id)
	nm.votingNeighbors.maybeRemove(id)
}

func (nm *neighborNodes) GetConnectionCnt() uint {
	return uint(len(nm.GetNeighbors(nil)))
}

func (nm *neighborNodes) GetNeighborHeights() ([]uint32, uint) {
	neighbors := nm.GetNeighbors(nil)
	heights := make([]uint32, 0, len(neighbors))
	for _, n := range neighbors {
		heights = append(heights, n.GetHeight())
	}
	return heights, uint(len(heights))
}

func (nm *neighborNodes) GetNeighbors(filter func(*RemoteNode) bool) []*RemoteNode {
	neighbors := make([]*RemoteNode, 0)
	nm.nodes.Range(func(key, value interface{}) bool {
		if rn, ok := value.(*RemoteNode); ok {
			if filter == nil || filter(rn) {
				neighbors = append(neighbors, rn)
			}
		}
		return true
	})
	return neighbors
}

func (localNode *LocalNode) getNeighborByNNetNode(nnetRemoteNode *nnetnode.RemoteNode) *RemoteNode {
	if nnetRemoteNode == nil {
		return nil
	}
	nbr := localNode.GetNeighborNode(chordIDToNodeID(nnetRemoteNode.Id))
	return nbr
}

func (localNode *LocalNode) computeNumRandomNeighbors(numRandomNeighborsFactor, minNumRandomNeighbors int) int {
	c, ok := localNode.nnet.Network.(*chord.Chord)
	if !ok {
		log.Fatal("Overlay is not chord")
	}

	numRandomNeighbors := 0
	for _, finger := range c.FingerTable() {
		if len(finger) > 0 {
			numRandomNeighbors += numRandomNeighborsFactor
		}
	}

	if numRandomNeighbors > minNumRandomNeighbors {
		return numRandomNeighbors
	}

	return minNumRandomNeighbors
}

func (localNode *LocalNode) connectToRandomNeighbors(rn *randomNeighbors, numRandomNeighborsFactor, minNumRandomNeighbors int) {
	c, ok := localNode.nnet.Network.(*chord.Chord)
	if !ok {
		log.Fatal("Overlay is not chord")
	}

	for {
		connectTimer := time.After(rn.connectInterval)
		for {
			rn.RLock()
			n := len(rn.ids)
			rn.RUnlock()

			if n < localNode.computeNumRandomNeighbors(numRandomNeighborsFactor, minNumRandomNeighbors) {
				break
			}

			select {
			case id := <-rn.removeChan:
				rn.Lock()
				for i := 0; i < len(rn.ids); i++ {
					if rn.ids[i] == id {
						rn.ids = append(rn.ids[:i], rn.ids[i+1:]...)
						i--
					}
				}
				rn.Unlock()
			case <-connectTimer:
				rn.Lock()
				if len(rn.ids) >= localNode.computeNumRandomNeighbors(numRandomNeighborsFactor, minNumRandomNeighbors) {
					nbr := localNode.GetNeighborNode(rn.ids[0])
					if nbr != nil {
						c.MaybeStopRemoteNode(nbr.nnetNode)
					}
					rn.ids = rn.ids[1:]
				}
				rn.Unlock()
				connectTimer = time.After(rn.connectInterval)
			}
		}

		randID := util.RandomBytes(config.NodeIDBytes)

		succs, err := c.FindSuccessors(randID, 1)
		if err != nil {
			log.Infof("Find random neighbor at key %x error: %v", randID, err)
			time.Sleep(time.Second)
			continue
		}

		if len(succs) == 0 {
			log.Warningf("Find no random neighbor at key %x", randID)
			time.Sleep(time.Second)
			continue
		}

		err = c.Connect(succs[0])
		if err != nil {
			log.Infof("Connect to random neighbor at key %x error: %v", randID, err)
			time.Sleep(time.Second)
			continue
		}

		log.Infof("Connect to random neighbor %x@%s", succs[0].Id, succs[0].Addr)

		rn.Lock()
		rn.ids = append(rn.ids, chordIDToNodeID(succs[0].Id))
		rn.Unlock()
	}
}

func (localNode *LocalNode) startConnectingToRandomNeighbors() {
	time.Sleep(randomNeighborsConnectDelay)
	go localNode.connectToRandomNeighbors(localNode.gossipNeighbors, config.NumRandomGossipNeighborsFactor, config.MinNumRandomGossipNeighbors)
	go localNode.connectToRandomNeighbors(localNode.votingNeighbors, config.NumRandomVotingNeighborsFactor, config.MinNumRandomVotingNeighbors)
}

func (localNode *LocalNode) getRandomNeighbors(rn *randomNeighbors, filter func(*RemoteNode) bool) []*RemoteNode {
	rn.RLock()
	defer rn.RUnlock()

	neighbors := make([]*RemoteNode, 0, len(rn.ids))
	for _, id := range rn.ids {
		nbr := localNode.GetNeighborNode(id)
		if nbr != nil {
			if filter == nil || filter(nbr) {
				neighbors = append(neighbors, nbr)
			}
		}
	}

	return neighbors
}

func (localNode *LocalNode) splitNeighbors(filter func(*RemoteNode) bool) ([]*RemoteNode, []*RemoteNode) {
	c, ok := localNode.nnet.Network.(*chord.Chord)
	if !ok {
		log.Fatal("Overlay is not chord")
	}

	allNeighbors := localNode.GetNeighbors(filter)
	chordNeighborIDs := make(map[string]struct{}, len(allNeighbors))

	for _, succ := range c.Successors() {
		chordNeighborIDs[string(succ.Id)] = struct{}{}
	}

	for _, pred := range c.Predecessors() {
		chordNeighborIDs[string(pred.Id)] = struct{}{}
	}

	for _, fingers := range c.FingerTable() {
		for _, finger := range fingers {
			chordNeighborIDs[string(finger.Id)] = struct{}{}
		}
	}

	chordNeighbors := make([]*RemoteNode, 0, len(allNeighbors))
	randomNeighbors := make([]*RemoteNode, 0, len(allNeighbors))

	for _, neighbor := range allNeighbors {
		if neighbor.Id == nil {
			continue
		}

		if _, ok := chordNeighborIDs[string(neighbor.Id)]; ok {
			chordNeighbors = append(chordNeighbors, neighbor)
			continue
		}

		if fingerIdx, _ := c.FingerTableIdxInRemoteNode(neighbor.Id); fingerIdx >= 0 {
			chordNeighbors = append(chordNeighbors, neighbor)
			continue
		}

		randomNeighbors = append(randomNeighbors, neighbor)
	}

	return chordNeighbors, randomNeighbors
}

func (localNode *LocalNode) getSampledNeighbors(rn *randomNeighbors, chordNeighborSampleRate float64, chordNeighborMinSample int, filter func(*RemoteNode) bool) []*RemoteNode {
	sampledNeighbors := localNode.getRandomNeighbors(rn, filter)

	if chordNeighborSampleRate > 0 || chordNeighborMinSample > 0 {
		chordNeighbors, _ := localNode.splitNeighbors(filter)
		numChordSamples := int(chordNeighborSampleRate * float64(len(chordNeighbors)))
		if numChordSamples < chordNeighborMinSample {
			numChordSamples = chordNeighborMinSample
		}
		if numChordSamples > len(chordNeighbors) {
			numChordSamples = len(chordNeighbors)
		}
		if numChordSamples > 0 {
			rand.Shuffle(len(chordNeighbors), func(i, j int) { chordNeighbors[i], chordNeighbors[j] = chordNeighbors[j], chordNeighbors[i] })
			sampledNeighbors = append(sampledNeighbors, chordNeighbors[:numChordSamples]...)
		}
	}

	return sampledNeighbors
}

func (localNode *LocalNode) GetGossipNeighbors(filter func(*RemoteNode) bool) []*RemoteNode {
	return localNode.getSampledNeighbors(localNode.gossipNeighbors, config.GossipSampleChordNeighbor, config.GossipMinChordNeighbor, filter)
}

func (localNode *LocalNode) GetVotingNeighbors(filter func(*RemoteNode) bool) []*RemoteNode {
	return localNode.getSampledNeighbors(localNode.votingNeighbors, config.VotingSampleChordNeighbor, config.VotingMinChordNeighbor, filter)
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

		height := localNode.GetHeight()
		if localNode.GetSyncState() != pb.SyncState_PERSIST_FINISHED {
			height = uint32(math.MaxUint32)
		}

		id, err := chain.DefaultLedger.Store.GetID(nodeData.PublicKey, height)
		if err != nil || len(id) == 0 || bytes.Equal(id, crypto.Sha256ZeroHash) {
			if localNode.GetSyncState() == pb.SyncState_PERSIST_FINISHED {
				return fmt.Errorf("remote node id can not be found in local ledger: err-%v, id-%v", err, id)
			}
		} else {
			if !bytes.Equal(id, n.GetId()) {
				return fmt.Errorf("remote node id should be %x instead of %x", id, n.GetId())
			}
		}
	}

	if ShouldRejectAddr(localNode.GetAddr(), n.GetAddr()) {
		return errors.New("remote port is different from local port")
	}

	return nil
}

func (localNode *LocalNode) verifyRemoteNode(remoteNode *nnetnode.RemoteNode) error {
	if remoteNode.GetId() == nil {
		return errors.New("remote node id is nil")
	}

	if remoteNode.GetData() == nil {
		return errors.New("remote node data is nil")
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
		return fmt.Errorf("remote node host %s is different from its connection host %s", addr.Hostname(), connHost)
	}

	if remoteNode.IsOutbound && addr.Port() != connPort {
		return fmt.Errorf("remote node port %v is different from its connection port %v", addr.Port(), connPort)
	}

	return nil
}

func (localNode *LocalNode) verifyNeighbors() {
	for _, nbr := range localNode.GetNeighbors(nil) {
		err := localNode.verifyRemoteNode(nbr.nnetNode)
		if err != nil {
			nbr.nnetNode.Stop(err)
		}
	}
}

func (localNode *LocalNode) addRemoteNode(nnetNode *nnetnode.RemoteNode) error {
	remoteNode, err := NewRemoteNode(localNode, nnetNode)
	if err != nil {
		return err
	}

	localNode.addNeighborNode(remoteNode)

	return nil
}

func (localNode *LocalNode) maybeAddRemoteNode(remoteNode *nnetnode.RemoteNode) error {
	if remoteNode == nil {
		return nil
	}

	if localNode.getNeighborByNNetNode(remoteNode) != nil {
		return nil
	}

	err := localNode.addRemoteNode(remoteNode)
	if err != nil {
		return err
	}

	if !remoteNode.IsOutbound {
		_, inboundRandomNeighbors := localNode.splitNeighbors(func(rn *RemoteNode) bool {
			return !rn.nnetNode.IsOutbound
		})
		if len(inboundRandomNeighbors) > config.MaxNumInboundRandomNeighbors {
			for _, inboundRandomNeighbor := range inboundRandomNeighbors {
				if bytes.Equal(inboundRandomNeighbor.Id, remoteNode.Id) {
					return fmt.Errorf("node has %d/%d inbound random neighbors", len(inboundRandomNeighbors), config.MaxNumInboundRandomNeighbors)
				}
			}
		}
	}

	return nil
}

func (localNode *LocalNode) VerifySigChain(sc *pb.SigChain, height uint32) error {
	if !config.SigChainObjection.GetValueAtHeight(height) {
		return nil
	}

	c, ok := localNode.nnet.Network.(*chord.Chord)
	if !ok {
		log.Fatal("Overlay is not chord")
	}

	if config.SigChainVerifySkipNode.GetValueAtHeight(height) && localNode.GetSyncState() == pb.SyncState_PERSIST_FINISHED {
		// only needs to verify node to node hop, and no need to check last node to
		// node hop because it could be successor
		for i := 1; i < sc.Length()-3; i++ {
			dist := chord.Distance(sc.Elems[i].Id, sc.Elems[i+1].Id, config.NodeIDBytes*8)
			fingerIdx := dist.BitLen() - 1
			fingerStartID := chord.PowerOffset(sc.Elems[i].Id, uint32(fingerIdx), config.NodeIDBytes*8)
			if !chord.BetweenLeftIncl(fingerStartID, sc.Elems[i+1].Id, localNode.Id) {
				continue
			}
			skipped := 1
			for _, succ := range c.Successors() {
				if skipped >= por.MaxNextHopChoice {
					break
				}
				rn := localNode.getNeighborByNNetNode(succ)
				if rn == nil {
					continue
				}
				if rn.GetSyncState() != pb.SyncState_PERSIST_FINISHED {
					continue
				}
				if chord.BetweenLeftIncl(fingerStartID, sc.Elems[i+1].Id, succ.Id) {
					skipped++
				}
			}
			for _, pred := range c.Predecessors() {
				if skipped >= por.MaxNextHopChoice {
					break
				}
				rn := localNode.getNeighborByNNetNode(pred)
				if rn == nil {
					continue
				}
				if rn.GetSyncState() != pb.SyncState_PERSIST_FINISHED {
					continue
				}
				if chord.BetweenLeftIncl(fingerStartID, sc.Elems[i+1].Id, pred.Id) {
					skipped++
				}
			}
			if skipped >= por.MaxNextHopChoice {
				return fmt.Errorf("skipped nodes")
			}
		}
	}

	return nil
}

func (localNode *LocalNode) VerifySigChainObjection(sc *pb.SigChain, reporterID []byte, height uint32) (int, error) {
	if !config.SigChainObjection.GetValueAtHeight(height) {
		return 0, fmt.Errorf("sigchain objection is not enabled")
	}

	if config.SigChainVerifySkipNode.GetValueAtHeight(height) {
		// only needs to verify node to node hop, and no need to check last node to
		// node hop because it could be successor
		for i := 1; i < sc.Length()-3; i++ {
			dist := chord.Distance(sc.Elems[i].Id, sc.Elems[i+1].Id, config.NodeIDBytes*8)
			fingerIdx := dist.BitLen() - 1
			fingerStartID := chord.PowerOffset(sc.Elems[i].Id, uint32(fingerIdx), config.NodeIDBytes*8)
			if chord.BetweenLeftIncl(fingerStartID, sc.Elems[i+1].Id, reporterID) {
				return i, nil
			}
		}
		return 0, fmt.Errorf("reporter is not skipped")
	}

	return 0, nil
}

// ShouldRejectAddr returns if remoteAddr should be rejected by localAddr
func ShouldRejectAddr(localAddr, remoteAddr string) bool {
	localAddress, err := url.Parse(localAddr)
	if err != nil {
		return false
	}

	remoteAddress, err := url.Parse(remoteAddr)
	if err != nil {
		return false
	}

	if localAddress.Hostname() != remoteAddress.Hostname() && localAddress.Port() != remoteAddress.Port() {
		return true
	}

	return false
}
