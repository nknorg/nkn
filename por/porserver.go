package por

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto/ed25519"
	"github.com/nknorg/nkn/event"
	"github.com/nknorg/nkn/pb"
	. "github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
)

const (
	sigChainTxnCacheExpiration           = 300 * time.Second
	sigChainTxnCacheCleanupInterval      = 10 * time.Second
	sigChainElemCacheExpiration          = 2 * config.ConsensusTimeout
	sigChainElemCacheCleanupInterval     = 10 * time.Second
	srcSigChainCacheExpiration           = 2 * config.ConsensusTimeout
	srcSigChainCacheCleanupInterval      = 10 * time.Second
	vrfCacheExpiration                   = (SigChainMiningHeightOffset + SigChainBlockHeightOffset + 5) * config.ConsensusTimeout
	vrfCacheCleanupInterval              = 10 * time.Second
	finalizedBlockCacheExpiration        = time.Hour
	finalizedBlockCacheCleanupInterval   = time.Minute
	miningPorPackageCacheExpiration      = time.Hour
	miningPorPackageCacheCleanupInterval = time.Minute
	destSigChainElemCacheExpiration      = time.Hour
	destSigChainElemCacheCleanupInterval = time.Minute
	flushSigChainDelay                   = 500 * time.Millisecond
)

type PorServer struct {
	account             *vault.Account
	id                  []byte
	sigChainTxnCache    common.Cache
	sigChainElemCache   common.Cache
	srcSigChainCache    common.Cache
	vrfCache            common.Cache
	finalizedBlockCache common.Cache

	sync.RWMutex
	miningPorPackageCache common.Cache
	destSigChainElemCache common.Cache
}

type vrfResult struct {
	vrf   []byte
	proof []byte
}

type sigChainElemInfo struct {
	nextPubkey    []byte
	prevNodeID    []byte
	prevSignature []byte
	mining        bool
	blockHash     []byte
}

type destSigChainElem struct {
	sigHash       []byte
	sigChainElem  *pb.SigChainElem
	prevSignature []byte
}

type BacktrackSigChainInfo struct {
	DestSigChainElem *pb.SigChainElem
	PrevSignature    []byte
}

var porServer *PorServer

func NewPorServer(account *vault.Account, id []byte) *PorServer {
	ps := &PorServer{
		account:               account,
		id:                    id,
		sigChainTxnCache:      common.NewGoCache(sigChainTxnCacheExpiration, sigChainTxnCacheCleanupInterval),
		sigChainElemCache:     common.NewGoCache(sigChainElemCacheExpiration, sigChainElemCacheCleanupInterval),
		srcSigChainCache:      common.NewGoCache(srcSigChainCacheExpiration, srcSigChainCacheCleanupInterval),
		vrfCache:              common.NewGoCache(vrfCacheExpiration, vrfCacheCleanupInterval),
		finalizedBlockCache:   common.NewGoCache(finalizedBlockCacheExpiration, finalizedBlockCacheCleanupInterval),
		miningPorPackageCache: common.NewGoCache(miningPorPackageCacheExpiration, miningPorPackageCacheCleanupInterval),
		destSigChainElemCache: common.NewGoCache(destSigChainElemCacheExpiration, destSigChainElemCacheCleanupInterval),
	}
	return ps
}

func InitPorServer(account *vault.Account, id []byte) error {
	if porServer != nil {
		return errors.New("PorServer already initialized")
	}
	if id == nil || len(id) == 0 {
		return errors.New("ID is empty")
	}
	porServer = NewPorServer(account, id)
	return nil
}

func GetPorServer() *PorServer {
	if porServer == nil {
		log.Error("PorServer not initialized")
		panic("PorServer not initialized")
	}
	return porServer
}

func (ps *PorServer) GetOrComputeVrf(data []byte) ([]byte, []byte, error) {
	if v, ok := ps.vrfCache.Get(data); ok {
		if res, ok := v.(*vrfResult); ok {
			return res.vrf, res.proof, nil
		}
	}

	vrf, proof, err := ed25519.GenerateVrf(ps.account.PrivKey(), data)
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

func (ps *PorServer) Sign(relayMessage *pb.Relay, nextPubkey, prevNodeID []byte, mining bool) error {
	vrf, _, err := ps.GetOrComputeVrf(relayMessage.BlockHash)
	if err != nil {
		log.Error("Get or compute VRF error:", err)
		return err
	}

	signature, err := pb.ComputeSignature(vrf, relayMessage.LastSignature, ps.id, nextPubkey, mining)
	if err != nil {
		log.Error("Computing signature error:", err)
		return err
	}

	ps.sigChainElemCache.Add(signature, &sigChainElemInfo{
		nextPubkey:    nextPubkey,
		prevNodeID:    prevNodeID,
		prevSignature: relayMessage.LastSignature,
		mining:        mining,
		blockHash:     relayMessage.BlockHash,
	})

	relayMessage.LastSignature = signature
	relayMessage.SigChainLen++

	return nil
}

func (ps *PorServer) Verify(sc *pb.SigChain) error {
	if err := sc.Verify(); err != nil {
		return err
	}

	return nil
}

func (ps *PorServer) CreateSigChainForClient(dataSize uint32, blockHash *common.Uint256, srcID,
	srcPubkey, destPubkey, signature []byte, sigAlgo pb.SigAlgo) (*pb.SigChain, error) {
	pubKey, err := ps.account.PubKey().EncodePoint(true)
	if err != nil {
		log.Error("Get account public key error:", err)
		return nil, err
	}
	sigChain, err := pb.NewSigChainWithSignature(dataSize, blockHash[:], srcID, srcPubkey, destPubkey, pubKey, signature, sigAlgo, false)
	if err != nil {
		log.Error("New signature chain with signature error:", err)
		return nil, err
	}
	ps.srcSigChainCache.Add(signature, sigChain)
	return sigChain, nil
}

func (ps *PorServer) IsFinal(sc *pb.SigChain) bool {
	return sc.IsFinal()
}

func (ps *PorServer) GetSignature(sc *pb.SigChain) ([]byte, error) {
	return sc.GetSignature()
}

func (ps *PorServer) LenOfSigChain(sc *pb.SigChain) int {
	return sc.Length()
}

func (ps *PorServer) GetMiningSigChain(height uint32) (*pb.SigChain, error) {
	if v, ok := ps.miningPorPackageCache.Get([]byte(strconv.Itoa(int(height)))); ok {
		if miningPorPackage, ok := v.(*PorPackage); ok {
			return miningPorPackage.SigChain, nil
		}
	}

	return nil, nil
}

func (ps *PorServer) GetMiningSigChainTxnHash(height uint32) (common.Uint256, error) {
	if v, ok := ps.miningPorPackageCache.Get([]byte(strconv.Itoa(int(height)))); ok {
		if miningPorPackage, ok := v.(*PorPackage); ok {
			if _, ok := ps.sigChainTxnCache.Get(miningPorPackage.TxHash); ok {
				return common.Uint256ParseFromBytes(miningPorPackage.TxHash)
			}
		}
	}

	return common.EmptyUint256, nil
}

func (ps *PorServer) GetMiningSigChainTxn(txnHash common.Uint256) (*Transaction, error) {
	v, ok := ps.sigChainTxnCache.Get(txnHash[:])
	if !ok {
		return nil, fmt.Errorf("sigchain txn %s not found", txnHash.ToHexString())
	}

	txn, ok := v.(*Transaction)
	if !ok {
		return nil, fmt.Errorf("convert to sigchain txn %s error", txnHash.ToHexString())
	}

	return txn, nil
}

func (ps *PorServer) AddSigChainFromTx(txn *Transaction, currentHeight uint32) (bool, error) {
	porPkg, err := NewPorPackage(txn)
	if err != nil {
		return false, err
	}

	voteForHeight := porPkg.VoteForHeight
	if voteForHeight < currentHeight+SigChainPropagationHeightOffset {
		return false, fmt.Errorf("sigchain vote for height %d is less than %d", voteForHeight, currentHeight+2)
	}

	ps.Lock()
	defer ps.Unlock()

	if v, ok := ps.miningPorPackageCache.Get([]byte(strconv.Itoa(int(voteForHeight)))); ok {
		if currentMiningPorPkg, ok := v.(*PorPackage); ok {
			if bytes.Compare(porPkg.SigHash, currentMiningPorPkg.SigHash) >= 0 {
				return false, nil
			}
		}
	}

	err = ps.sigChainTxnCache.Add(porPkg.TxHash, txn)
	if err != nil {
		return false, err
	}

	err = ps.miningPorPackageCache.Set([]byte(strconv.Itoa(int(voteForHeight))), porPkg)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (ps *PorServer) ShouldSignDestSigChainElem(blockHash, lastSignature []byte, sigChainLen int) bool {
	if _, ok := ps.finalizedBlockCache.Get(blockHash); ok {
		return false
	}
	if v, ok := ps.destSigChainElemCache.Get(blockHash); ok {
		if currentDestSigChainElem, ok := v.(*destSigChainElem); ok {
			sigHash := pb.ComputeSignatureHash(lastSignature, sigChainLen)
			if bytes.Compare(sigHash, currentDestSigChainElem.sigHash) >= 0 {
				return false
			}
		}
	}
	return true
}

func (ps *PorServer) AddDestSigChainElem(blockHash, lastSignature []byte, sigChainLen int, destElem *pb.SigChainElem) (bool, error) {
	ps.Lock()
	defer ps.Unlock()

	if !ps.ShouldSignDestSigChainElem(blockHash, lastSignature, sigChainLen) {
		return false, nil
	}

	err := ps.destSigChainElemCache.Set(blockHash, &destSigChainElem{
		sigHash:       pb.ComputeSignatureHash(lastSignature, sigChainLen),
		sigChainElem:  destElem,
		prevSignature: lastSignature,
	})
	if err != nil {
		return false, err
	}

	return true, nil
}

func (ps *PorServer) BacktrackSigChain(elems []*pb.SigChainElem, signature, senderPubkey []byte) ([]*pb.SigChainElem, []byte, []byte, error) {
	v, ok := ps.sigChainElemCache.Get(signature)
	if !ok {
		return nil, nil, nil, fmt.Errorf("sigchain element with signature %x not found", signature)
	}

	scei, ok := v.(*sigChainElemInfo)
	if !ok {
		return nil, nil, nil, fmt.Errorf("failed to decode cached sigchain element info")
	}

	if senderPubkey != nil && !bytes.Equal(senderPubkey, scei.nextPubkey) {
		return nil, nil, nil, fmt.Errorf("sender pubkey %x is different from expected value %x", senderPubkey, scei.nextPubkey)
	}

	if _, ok = ps.finalizedBlockCache.Get(scei.blockHash); !ok {
		return nil, nil, nil, fmt.Errorf("block %x is not finalized yet")
	}

	vrf, proof, err := ps.GetOrComputeVrf(scei.blockHash)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get or compute VRF error: %v", err)
	}

	sce := pb.NewSigChainElem(ps.id, scei.nextPubkey, signature, vrf, proof, scei.mining)

	elems = append([]*pb.SigChainElem{sce}, elems...)

	return elems, scei.prevSignature, scei.prevNodeID, nil
}

func (ps *PorServer) GetSrcSigChainFromCache(signature []byte) (*pb.SigChain, error) {
	v, ok := ps.srcSigChainCache.Get(signature)
	if !ok {
		return nil, fmt.Errorf("src sigchain with signature %x not found", signature)
	}

	sigChain, ok := v.(*pb.SigChain)
	if !ok {
		return nil, fmt.Errorf("failed to decode cached src sigchain from")
	}

	return sigChain, nil
}

func (ps *PorServer) FlushSigChain(blockHash []byte) {
	ps.Lock()
	defer ps.Unlock()

	ps.finalizedBlockCache.Add(blockHash, struct{}{})

	if v, ok := ps.destSigChainElemCache.Get(blockHash); ok {
		if sce, ok := v.(*destSigChainElem); ok {
			time.AfterFunc(util.RandDuration(flushSigChainDelay, 0.5), func() {
				event.Queue.Notify(event.BacktrackSigChain, &BacktrackSigChainInfo{
					DestSigChainElem: sce.sigChainElem,
					PrevSignature:    sce.prevSignature,
				})
			})
		}
	}
}
