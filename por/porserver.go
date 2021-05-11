package por

import (
	"bytes"
	"container/heap"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/nknorg/nkn/v2/block"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/crypto"
	"github.com/nknorg/nkn/v2/event"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/transaction"
	"github.com/nknorg/nkn/v2/util"
	"github.com/nknorg/nkn/v2/util/log"
	"github.com/nknorg/nkn/v2/vault"
)

const (
	sigChainElemCacheExpiration          = 10 * time.Second
	sigChainElemCachePinnedExpiration    = 10 * config.ConsensusTimeout
	sigChainElemCacheCleanupInterval     = 2 * time.Second
	srcSigChainCacheExpiration           = 10 * time.Second
	srcSigChainCachePinnedExpiration     = 10 * config.ConsensusTimeout
	srcSigChainCacheCleanupInterval      = 2 * time.Second
	destSigChainElemCacheExpiration      = 10 * config.ConsensusTimeout
	destSigChainElemCacheCleanupInterval = config.ConsensusDuration
	finalizedBlockCacheExpiration        = 10 * config.ConsensusTimeout
	finalizedBlockCacheCleanupInterval   = config.ConsensusDuration
	sigChainTxnCacheExpiration           = 10 * config.ConsensusTimeout
	sigChainTxnCacheCleanupInterval      = config.ConsensusDuration
	miningPorPackageCacheExpiration      = 10 * config.ConsensusTimeout
	miningPorPackageCacheCleanupInterval = config.ConsensusDuration
	vrfCacheExpiration                   = (SigChainMiningHeightOffset + config.SigChainBlockDelay + 5) * config.ConsensusTimeout
	vrfCacheCleanupInterval              = config.ConsensusDuration
	flushSigChainDelay                   = 500 * time.Millisecond
	maxNumSigChainInCache                = 8
	MaxNextHopChoice                     = 4 // should be >= nnet NumFingerSuccessors to avoid false positive
)

type PorServer struct {
	account                   *vault.Account
	id                        []byte
	sigChainTxnCache          common.Cache
	sigChainTxnSigHashCache   common.Cache
	sigChainTxnShortHashCache common.Cache
	sigChainElemCache         common.Cache
	srcSigChainCache          common.Cache
	vrfCache                  common.Cache
	finalizedBlockCache       common.Cache

	sync.RWMutex
	miningPorPackageCache  common.Cache
	destSigChainElemCache  common.Cache
	sigChainObjectionCache common.Cache
}

// Store interface is used to avoid cyclic dependency
type Store interface {
	GetHeightByBlockHash(hash common.Uint256) (uint32, error)
	GetHeaderWithCache(hash common.Uint256) (*block.Header, error)
	GetSigChainWithCache(hash common.Uint256) (*pb.SigChain, error)
	GetID(publicKey []byte, height uint32) ([]byte, error)
}

var store Store

// LocalNode interface is used to avoid cyclic dependency
type LocalNode interface {
	VerifySigChainObjection(sc *pb.SigChain, reporterID []byte, height uint32) (int, error)
}

var localNode LocalNode

type vrfResult struct {
	vrf   []byte
	proof []byte
}

type sigChainElemInfo struct {
	nextPubkey  []byte
	prevNodeID  []byte
	prevHash    []byte
	blockHash   []byte
	mining      bool
	pinned      bool
	backtracked bool
	sigAlgo     pb.SigAlgo
}

type srcSigChain struct {
	sigChain *pb.SigChain
	pinned   bool
}

type destSigChainElem struct {
	sigHash      []byte
	sigChainElem *pb.SigChainElem
	prevHash     []byte
}

type PinSigChainInfo struct {
	PrevHash []byte
}

type BacktrackSigChainInfo struct {
	DestSigChainElem *pb.SigChainElem
	PrevHash         []byte
}

type sigChainObjection struct {
	reporterPubkey []byte
	reporterID     []byte
	skippedHop     int
}

type sigChainObjections []*sigChainObjection

var porServer *PorServer

type porPackageHeap []*PorPackage

func (s porPackageHeap) Len() int            { return len(s) }
func (s porPackageHeap) Swap(i, j int)       { s[i], s[j] = s[j], s[i] }
func (s porPackageHeap) Less(i, j int) bool  { return bytes.Compare(s[i].SigHash, s[j].SigHash) < 0 }
func (s *porPackageHeap) Push(x interface{}) { *s = append(*s, x.(*PorPackage)) }
func (s *porPackageHeap) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

func NewPorServer(account *vault.Account, id []byte) *PorServer {
	ps := &PorServer{
		account:                   account,
		id:                        id,
		sigChainTxnCache:          common.NewGoCache(sigChainTxnCacheExpiration, sigChainTxnCacheCleanupInterval),
		sigChainTxnSigHashCache:   common.NewGoCache(sigChainTxnCacheExpiration, sigChainTxnCacheCleanupInterval),
		sigChainTxnShortHashCache: common.NewGoCache(sigChainTxnCacheExpiration, sigChainTxnCacheCleanupInterval),
		sigChainElemCache:         common.NewGoCache(sigChainElemCacheExpiration, sigChainElemCacheCleanupInterval),
		srcSigChainCache:          common.NewGoCache(srcSigChainCacheExpiration, srcSigChainCacheCleanupInterval),
		vrfCache:                  common.NewGoCache(vrfCacheExpiration, vrfCacheCleanupInterval),
		finalizedBlockCache:       common.NewGoCache(finalizedBlockCacheExpiration, finalizedBlockCacheCleanupInterval),
		miningPorPackageCache:     common.NewGoCache(miningPorPackageCacheExpiration, miningPorPackageCacheCleanupInterval),
		destSigChainElemCache:     common.NewGoCache(destSigChainElemCacheExpiration, destSigChainElemCacheCleanupInterval),
		sigChainObjectionCache:    common.NewGoCache(miningPorPackageCacheExpiration, miningPorPackageCacheCleanupInterval),
	}
	return ps
}

func InitPorServer(account *vault.Account, id []byte, s Store, l LocalNode) error {
	if porServer != nil {
		return errors.New("PorServer already initialized")
	}
	if id == nil || len(id) == 0 {
		return errors.New("ID is empty")
	}
	porServer = NewPorServer(account, id)
	store = s
	localNode = l
	return nil
}

func GetPorServer() *PorServer {
	if porServer == nil {
		log.Fatal("PorServer not initialized")
	}
	return porServer
}

func (ps *PorServer) GetOrComputeVrf(data []byte) ([]byte, []byte, error) {
	if v, ok := ps.vrfCache.Get(data); ok {
		if res, ok := v.(*vrfResult); ok {
			return res.vrf, res.proof, nil
		}
	}

	vrf, proof, err := crypto.GenerateVrf(ps.account.PrivKey(), data, false)
	if err != nil {
		return nil, nil, err
	}

	res := &vrfResult{
		vrf:   vrf,
		proof: proof,
	}

	ps.vrfCache.Add(data, res)

	return vrf, proof, nil
}

func (ps *PorServer) UpdateRelayMessage(relayMessage *pb.Relay, nextPubkey, prevNodeID []byte, mining bool) error {
	vrf, _, err := ps.GetOrComputeVrf(relayMessage.BlockHash)
	if err != nil {
		return err
	}

	blockHash, err := common.Uint256ParseFromBytes(relayMessage.BlockHash)
	if err != nil {
		return err
	}

	sigAlgo := pb.SigAlgo_HASH
	height, err := store.GetHeightByBlockHash(blockHash)
	if err == nil {
		if !config.AllowSigChainHashSignature.GetValueAtHeight(height) {
			sigAlgo = pb.SigAlgo_SIGNATURE
		}
	}

	sce := pb.NewSigChainElem(ps.id, nextPubkey, nil, vrf, nil, mining, sigAlgo)
	hash, err := sce.Hash(relayMessage.LastHash)
	if err != nil {
		return err
	}

	ps.sigChainElemCache.Add(hash, &sigChainElemInfo{
		nextPubkey:  nextPubkey,
		prevNodeID:  prevNodeID,
		prevHash:    relayMessage.LastHash,
		blockHash:   relayMessage.BlockHash,
		mining:      mining,
		backtracked: false,
		sigAlgo:     sigAlgo,
	})

	relayMessage.LastHash = hash
	relayMessage.SigChainLen++

	return nil
}

func (ps *PorServer) CreateSigChainForClient(nonce, dataSize uint32, blockHash []byte, srcID, srcPubkey, destID, destPubkey, signature []byte, sigAlgo pb.SigAlgo) (*pb.SigChain, error) {
	pubKey := ps.account.PubKey()
	sigChain, err := pb.NewSigChain(
		nonce,
		dataSize,
		blockHash,
		srcID,
		srcPubkey,
		destID,
		destPubkey,
		pubKey,
		signature,
		sigAlgo,
		false,
	)
	if err != nil {
		return nil, err
	}
	ps.srcSigChainCache.Add(signature, &srcSigChain{sigChain: sigChain})
	return sigChain, nil
}

func (ps *PorServer) GetMiningSigChainTxnHash(voteForHeight uint32) (common.Uint256, error) {
	ps.RLock()
	defer ps.RUnlock()

	var height uint32
	if voteForHeight > SigChainMiningHeightOffset+config.SigChainBlockDelay {
		height = voteForHeight - SigChainMiningHeightOffset - config.SigChainBlockDelay
	}

	if v, ok := ps.miningPorPackageCache.Get([]byte(strconv.Itoa(int(voteForHeight)))); ok {
		if p, ok := v.(*porPackageHeap); ok {
			for i := 0; i < p.Len(); i++ {
				porPkg := (*p)[i]
				if v, ok := ps.sigChainObjectionCache.Get(porPkg.SigHash); ok {
					if scos, ok := v.(sigChainObjections); ok {
						if len(scos) >= MaxNextHopChoice {
							verifiedCount := make(map[int]int)
							for _, sco := range scos {
								if len(sco.reporterID) == 0 {
									id, err := store.GetID(sco.reporterPubkey, height)
									if err != nil {
										continue
									}
									sco.reporterID = id
									i, err := localNode.VerifySigChainObjection(porPkg.SigChain, id, height)
									if err != nil {
										continue
									}
									sco.skippedHop = i
								}
								if sco.skippedHop > 0 {
									verifiedCount[sco.skippedHop]++
								}
								if verifiedCount[sco.skippedHop] >= MaxNextHopChoice {
									break
								}
							}
							isSigChainInvalid := false
							for _, count := range verifiedCount {
								if count >= MaxNextHopChoice {
									isSigChainInvalid = true
									break
								}
							}
							if isSigChainInvalid {
								continue
							}
						}
					}
				}
				if _, ok := ps.sigChainTxnCache.Get(porPkg.TxHash); ok {
					return common.Uint256ParseFromBytes(porPkg.TxHash)
				}
			}
		}
	}

	return common.EmptyUint256, nil
}

func (ps *PorServer) GetSigChainTxn(txnHash common.Uint256) (*transaction.Transaction, error) {
	v, ok := ps.sigChainTxnCache.Get(txnHash[:])
	if !ok {
		return nil, fmt.Errorf("sigchain txn %s not found", txnHash.ToHexString())
	}

	txn, ok := v.(*transaction.Transaction)
	if !ok {
		return nil, fmt.Errorf("convert to sigchain txn %s error", txnHash.ToHexString())
	}

	return txn, nil
}

func (ps *PorServer) GetSigChainTxnBySigHash(sigHash []byte) (*transaction.Transaction, error) {
	v, ok := ps.sigChainTxnSigHashCache.Get(sigHash)
	if !ok {
		return nil, fmt.Errorf("sigchain txn sig hash %x not found", sigHash)
	}

	txn, ok := v.(*transaction.Transaction)
	if !ok {
		return nil, fmt.Errorf("convert to sigchain txn %x error", sigHash)
	}

	return txn, nil
}

func (ps *PorServer) GetSigChainTxnByShortHash(shortHash []byte) (*transaction.Transaction, error) {
	v, ok := ps.sigChainTxnShortHashCache.Get(shortHash)
	if !ok {
		return nil, fmt.Errorf("sigchain txn short hash %x not found", shortHash)
	}

	txn, ok := v.(*transaction.Transaction)
	if !ok {
		return nil, fmt.Errorf("convert to sigchain txn %x error", shortHash)
	}

	return txn, nil
}

func (ps *PorServer) ShouldAddSigChainToCache(currentHeight, voteForHeight uint32, sigHash []byte, replace bool) bool {
	ps.RLock()
	defer ps.RUnlock()
	return ps.shouldAddSigChainToCache(currentHeight, voteForHeight, sigHash, replace)
}

func (ps *PorServer) shouldAddSigChainToCache(currentHeight, voteForHeight uint32, sigHash []byte, replace bool) bool {
	if voteForHeight < currentHeight+SigChainPropagationHeightOffset {
		return false
	}

	if voteForHeight > currentHeight+SigChainMiningHeightOffset {
		return false
	}

	if v, ok := ps.miningPorPackageCache.Get([]byte(strconv.Itoa(int(voteForHeight)))); ok {
		if p, ok := v.(*porPackageHeap); ok {
			if replace {
				return bytes.Compare(sigHash, (*p)[p.Len()-1].SigHash) <= 0
			}
			return bytes.Compare(sigHash, (*p)[p.Len()-1].SigHash) < 0
		}
	}

	return true
}

func VerifyID(sc *pb.SigChain, height uint32) error {
	for i := range sc.Elems {
		if i == 0 || (sc.IsComplete() && i == sc.Length()-1) {
			continue
		}

		pk := sc.Elems[i-1].NextPubkey
		id, err := store.GetID(pk, height)
		if err != nil {
			return fmt.Errorf("get id of pk %x error: %v", pk, err)
		}
		if len(id) == 0 {
			return fmt.Errorf("id of pk %x is empty", pk)
		}
		if !bytes.Equal(sc.Elems[i].Id, id) {
			return fmt.Errorf("id of pk %x should be %x, got %x", pk, id, sc.Elems[i].Id)
		}
	}
	return nil
}

func (ps *PorServer) AddSigChainFromTx(txn *transaction.Transaction, currentHeight uint32) (*PorPackage, error) {
	porPkg, err := NewPorPackage(txn)
	if err != nil {
		return nil, err
	}

	ps.Lock()
	defer ps.Unlock()

	if !ps.shouldAddSigChainToCache(currentHeight, porPkg.VoteForHeight, porPkg.SigHash, false) {
		return nil, nil
	}

	err = VerifyID(porPkg.SigChain, porPkg.Height)
	if err != nil {
		return nil, err
	}

	err = VerifySigChainSignatures(porPkg.SigChain)
	if err != nil {
		return nil, err
	}

	err = ps.sigChainTxnCache.Add(porPkg.TxHash, txn)
	if err != nil {
		return nil, err
	}

	err = ps.sigChainTxnSigHashCache.Add(porPkg.SigHash, txn)
	if err != nil {
		return nil, err
	}

	err = ps.sigChainTxnShortHashCache.Add(txn.ShortHash(config.ShortHashSalt, config.ShortHashSize), txn)
	if err != nil {
		return nil, err
	}

	var p *porPackageHeap
	if v, ok := ps.miningPorPackageCache.Get([]byte(strconv.Itoa(int(porPkg.VoteForHeight)))); ok {
		var ok bool
		if p, ok = v.(*porPackageHeap); ok {
			if p.Len() >= maxNumSigChainInCache {
				(*p)[p.Len()-1] = porPkg
				heap.Fix(p, p.Len()-1)
			} else {
				heap.Push(p, porPkg)
			}
		}
	}

	if p == nil {
		p = &porPackageHeap{porPkg}
		heap.Init(p)
	}

	err = ps.miningPorPackageCache.Set([]byte(strconv.Itoa(int(porPkg.VoteForHeight))), p)
	if err != nil {
		return nil, err
	}

	log.Debugf("Received better sigchain for height %d with sighash %x", porPkg.VoteForHeight, porPkg.SigHash)

	return porPkg, nil
}

func (ps *PorServer) ShouldSignDestSigChainElem(blockHash, lastHash []byte, sigChainLen int) bool {
	ps.RLock()
	defer ps.RUnlock()
	return ps.shouldSignDestSigChainElem(blockHash, lastHash, sigChainLen)
}

func (ps *PorServer) shouldSignDestSigChainElem(blockHash, lastHash []byte, sigChainLen int) bool {
	if _, ok := ps.finalizedBlockCache.Get(blockHash); ok {
		return false
	}

	blockHashUint256, err := common.Uint256ParseFromBytes(blockHash)
	if err != nil {
		return false
	}

	height, err := store.GetHeightByBlockHash(blockHashUint256)
	if err != nil {
		return false
	}

	if v, ok := ps.destSigChainElemCache.Get(blockHash); ok {
		if currentDestSigChainElem, ok := v.(*destSigChainElem); ok {
			sigHash := pb.ComputeSignatureHash(lastHash, sigChainLen, height, 0)
			if bytes.Compare(sigHash, currentDestSigChainElem.sigHash) >= 0 {
				return false
			}
		}
	}
	return true
}

func (ps *PorServer) AddDestSigChainElem(blockHash, lastHash []byte, sigChainLen int, destElem *pb.SigChainElem) (bool, error) {
	ps.Lock()
	defer ps.Unlock()

	if !ps.shouldSignDestSigChainElem(blockHash, lastHash, sigChainLen) {
		return false, nil
	}

	blockHashUint256, err := common.Uint256ParseFromBytes(blockHash)
	if err != nil {
		return false, err
	}

	height, err := store.GetHeightByBlockHash(blockHashUint256)
	if err != nil {
		return false, err
	}

	err = ps.destSigChainElemCache.Set(blockHash, &destSigChainElem{
		sigHash:      pb.ComputeSignatureHash(lastHash, sigChainLen, height, 0),
		sigChainElem: destElem,
		prevHash:     lastHash,
	})
	if err != nil {
		return false, err
	}

	event.Queue.Notify(event.PinSigChain, &PinSigChainInfo{
		PrevHash: lastHash,
	})

	return true, nil
}

// PinSigChain extends the cache expiration of a key
func (ps *PorServer) PinSigChain(hash, senderPubkey []byte) ([]byte, []byte, error) {
	v, ok := ps.sigChainElemCache.Get(hash)
	if !ok {
		return nil, nil, fmt.Errorf("sigchain element with hash %x not found", hash)
	}

	scei, ok := v.(*sigChainElemInfo)
	if !ok {
		return nil, nil, errors.New("failed to decode cached sigchain element info")
	}

	if scei.pinned {
		return nil, nil, errors.New("sigchain has already been pinned")
	}

	if senderPubkey != nil && !bytes.Equal(senderPubkey, scei.nextPubkey) {
		return nil, nil, fmt.Errorf("sender pubkey %x is different from expected value %x", senderPubkey, scei.nextPubkey)
	}

	err := ps.sigChainElemCache.SetWithExpiration(hash, scei, sigChainElemCachePinnedExpiration)
	if err != nil {
		return nil, nil, err
	}

	return scei.prevHash, scei.prevNodeID, nil
}

// PinSigChainSuccess marks a sigchain as pinned to avoid it being pinned
// multiple times.
func (ps *PorServer) PinSigChainSuccess(hash []byte) {
	if v, ok := ps.sigChainElemCache.Get(hash); ok {
		if scei, ok := v.(*sigChainElemInfo); ok {
			scei.pinned = true
		}
	}
}

// PinSrcSigChain marks a src sigchain as pinned to avoid it being pinned
// multiple times.
func (ps *PorServer) PinSrcSigChain(signature []byte) error {
	v, ok := ps.srcSigChainCache.Get(signature)
	if !ok {
		return fmt.Errorf("src sigchain with signature %x not found", signature)
	}

	ssc, ok := v.(*srcSigChain)
	if !ok {
		return fmt.Errorf("failed to decode cached src sigchain %x", signature)
	}

	if ssc.pinned {
		return errors.New("src sigchain has already been pinned")
	}

	ssc.pinned = true

	err := ps.srcSigChainCache.SetWithExpiration(signature, ssc, srcSigChainCachePinnedExpiration)
	if err != nil {
		return err
	}

	return nil
}

func (ps *PorServer) BacktrackSigChain(elems []*pb.SigChainElem, hash, senderPubkey []byte) ([]*pb.SigChainElem, []byte, []byte, error) {
	v, ok := ps.sigChainElemCache.Get(hash)
	if !ok {
		return nil, nil, nil, fmt.Errorf("sigchain element with hash %x not found", hash)
	}

	scei, ok := v.(*sigChainElemInfo)
	if !ok {
		return nil, nil, nil, errors.New("failed to decode cached sigchain element info")
	}

	if scei.backtracked {
		return nil, nil, nil, errors.New("sigchain has already been backtracked")
	}

	if senderPubkey != nil && !bytes.Equal(senderPubkey, scei.nextPubkey) {
		return nil, nil, nil, fmt.Errorf("sender pubkey %x is different from expected value %x", senderPubkey, scei.nextPubkey)
	}

	if _, ok = ps.finalizedBlockCache.Get(scei.blockHash); !ok {
		return nil, nil, nil, fmt.Errorf("block %x is not finalized yet", scei.blockHash)
	}

	vrf, proof, err := ps.GetOrComputeVrf(scei.blockHash)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get or compute VRF error: %v", err)
	}

	var signature []byte
	switch scei.sigAlgo {
	case pb.SigAlgo_SIGNATURE:
		signature, err = crypto.Sign(ps.account.PrivKey(), hash)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("sign error: %v", err)
		}
	case pb.SigAlgo_HASH:
		signature = hash
	default:
		return nil, nil, nil, fmt.Errorf("unknown sigAlgo: %v", scei.sigAlgo)
	}

	sce := pb.NewSigChainElem(ps.id, scei.nextPubkey, signature, vrf, proof, scei.mining, scei.sigAlgo)
	elems = append([]*pb.SigChainElem{sce}, elems...)

	return elems, scei.prevHash, scei.prevNodeID, nil
}

// BacktrackSigChainSuccess marks a sigchain as backtracked to avoid it being
// backtracked multiple times.
func (ps *PorServer) BacktrackSigChainSuccess(hash []byte) {
	if v, ok := ps.sigChainElemCache.Get(hash); ok {
		if scei, ok := v.(*sigChainElemInfo); ok {
			scei.backtracked = true
		}
	}
}

// PopSrcSigChainFromCache returns src sigchain and removes it from cache.
func (ps *PorServer) PopSrcSigChainFromCache(signature []byte) (*pb.SigChain, error) {
	v, ok := ps.srcSigChainCache.Get(signature)
	if !ok {
		return nil, fmt.Errorf("src sigchain with signature %x not found", signature)
	}

	ssc, ok := v.(*srcSigChain)
	if !ok {
		return nil, fmt.Errorf("failed to decode cached src sigchain %x", signature)
	}

	err := ps.srcSigChainCache.Delete(signature)
	if err != nil {
		return nil, err
	}

	return ssc.sigChain, nil
}

func (ps *PorServer) FlushSigChain(blockHash []byte) {
	ps.Lock()
	defer ps.Unlock()

	ps.finalizedBlockCache.Add(blockHash, struct{}{})

	if v, ok := ps.destSigChainElemCache.Get(blockHash); ok {
		if sce, ok := v.(*destSigChainElem); ok {
			time.Sleep(util.RandDuration(flushSigChainDelay, 0.5))
			log.Infof("Start backtracking sigchain with sighash %x", sce.sigHash)
			event.Queue.Notify(event.BacktrackSigChain, &BacktrackSigChainInfo{
				DestSigChainElem: sce.sigChainElem,
				PrevHash:         sce.prevHash,
			})
		}
	}
}

func (ps *PorServer) AddSigChainObjection(currentHeight, voteForHeight uint32, sigHash, reporterPubkey []byte) bool {
	var height uint32
	if voteForHeight > SigChainMiningHeightOffset+config.SigChainBlockDelay {
		height = voteForHeight - SigChainMiningHeightOffset - config.SigChainBlockDelay
	}
	if !config.SigChainObjection.GetValueAtHeight(height) {
		return false
	}

	ps.Lock()
	defer ps.Unlock()

	if !ps.shouldAddSigChainToCache(currentHeight, voteForHeight, sigHash, true) {
		return false
	}

	sco := &sigChainObjection{reporterPubkey: reporterPubkey}

	var porPkg *PorPackage
	if v, ok := ps.miningPorPackageCache.Get([]byte(strconv.Itoa(int(voteForHeight)))); ok {
		if p, ok := v.(*porPackageHeap); ok {
			for i := 0; i < p.Len(); i++ {
				if bytes.Compare((*p)[i].SigHash, sigHash) == 0 {
					porPkg = (*p)[i]
					break
				}
			}
		}
	}

	if porPkg != nil {
		reporterID, err := store.GetID(reporterPubkey, porPkg.Height)
		if err != nil {
			return false
		}
		sco.reporterID = reporterID
		i, err := localNode.VerifySigChainObjection(porPkg.SigChain, reporterID, porPkg.Height)
		if err != nil {
			return false
		}
		sco.skippedHop = i
	}

	var scos sigChainObjections
	if v, ok := ps.sigChainObjectionCache.Get(sigHash); ok {
		var ok bool
		if scos, ok = v.(sigChainObjections); ok {
			verifiedCount := make(map[int]int)
			needVerify := false
			for _, s := range scos {
				if bytes.Equal(s.reporterPubkey, reporterPubkey) {
					return false
				}
				if s.skippedHop > 0 {
					verifiedCount[s.skippedHop]++
				}
				if len(s.reporterID) == 0 {
					needVerify = true
				}
			}

			for _, count := range verifiedCount {
				if count >= MaxNextHopChoice {
					return false
				}
			}

			if porPkg != nil && needVerify {
				verifiedScos := sigChainObjections{}
				verifiedCount := make(map[int]int)
				for _, s := range scos {
					reporterID, err := store.GetID(s.reporterPubkey, porPkg.Height)
					if err != nil {
						continue
					}
					s.reporterID = reporterID
					i, err := localNode.VerifySigChainObjection(porPkg.SigChain, reporterID, porPkg.Height)
					if err != nil {
						continue
					}
					s.skippedHop = i
					verifiedScos = append(verifiedScos, s)
					verifiedCount[s.skippedHop]++
				}
				scos = verifiedScos

				for _, count := range verifiedCount {
					if count >= MaxNextHopChoice {
						ps.sigChainObjectionCache.Set(sigHash, scos)
						return false
					}
				}
			}

			scos = append(scos, sco)
		}
	}

	if len(scos) == 0 {
		scos = sigChainObjections{sco}
	}

	ps.sigChainObjectionCache.Set(sigHash, scos)

	return true
}
