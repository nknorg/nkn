package por

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
)

const (
	sigChainTxnCacheExpiration      = 300 * time.Second
	sigChainTxnCacheCleanupInterval = 10 * time.Second
)

type PorServer struct {
	account          *vault.Account
	id               []byte
	sigChainTxnCache common.Cache

	sync.RWMutex
	miningPorPackage map[uint32]*PorPackage
}

var porServer *PorServer

func NewPorServer(account *vault.Account, id []byte) *PorServer {
	ps := &PorServer{
		account:          account,
		id:               id,
		sigChainTxnCache: common.NewGoCache(sigChainTxnCacheExpiration, sigChainTxnCacheCleanupInterval),
		miningPorPackage: make(map[uint32]*PorPackage),
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

func (ps *PorServer) Sign(sc *SigChain, nextPubkey []byte, mining bool) error {
	dcPk, err := ps.account.PubKey().EncodePoint(true)
	if err != nil {
		log.Error("Get account public key error:", err)
		return err
	}

	nxPk, err := sc.GetLastPubkey()
	if err != nil {
		log.Error("Get last public key error:", err)
		return err
	}

	if !common.IsEqualBytes(dcPk, nxPk) {
		return errors.New("it's not the right signer")
	}

	err = sc.Sign(ps.id, nextPubkey, mining, ps.account)
	if err != nil {
		log.Error("Signature chain signing error:", err)
		return err
	}

	return nil
}

func (ps *PorServer) Verify(sc *SigChain) error {
	if err := sc.Verify(); err != nil {
		return err
	}

	return nil
}

func (ps *PorServer) CreateSigChain(dataSize uint32, dataHash, blockHash *common.Uint256, srcID,
	destPubkey, nextPubkey []byte, mining bool) (*SigChain, error) {
	return NewSigChain(ps.account, dataSize, dataHash[:], blockHash[:], srcID, destPubkey, nextPubkey, mining)
}

func (ps *PorServer) CreateSigChainForClient(dataSize uint32, dataHash, blockHash *common.Uint256, srcID,
	srcPubkey, destPubkey, signature []byte, sigAlgo SigAlgo) (*SigChain, error) {
	pubKey, err := ps.account.PubKey().EncodePoint(true)
	if err != nil {
		log.Error("Get account public key error:", err)
		return nil, err
	}
	sigChain, err := NewSigChainWithSignature(dataSize, dataHash[:], blockHash[:],
		srcID, srcPubkey, destPubkey, pubKey, signature, sigAlgo, false)
	if err != nil {
		log.Error("New signature chain with signature error:", err)
		return nil, err
	}
	return sigChain, nil
}

func (ps *PorServer) IsFinal(sc *SigChain) bool {
	return sc.IsFinal()
}

func (ps *PorServer) IsSatisfyThreshold() bool {
	//TODO need to add codes
	return true
}

func (ps *PorServer) GetSignature(sc *SigChain) ([]byte, error) {
	return sc.GetSignature()
}

func (ps *PorServer) LenOfSigChain(sc *SigChain) int {
	return sc.Length()
}

func (ps *PorServer) GetMiningSigChain(height uint32) (*SigChain, error) {
	ps.RLock()
	defer ps.RUnlock()

	miningPorPackage := ps.miningPorPackage[height]
	if miningPorPackage == nil {
		return nil, nil
	}

	return miningPorPackage.GetSigChain(), nil
}

func (ps *PorServer) GetMiningSigChainTxnHash(height uint32) (common.Uint256, error) {
	ps.RLock()
	defer ps.RUnlock()

	porPackage := ps.miningPorPackage[height]
	if porPackage == nil {
		return common.EmptyUint256, nil
	}

	if _, ok := ps.sigChainTxnCache.Get(porPackage.TxHash); !ok {
		return common.EmptyUint256, nil
	}

	return common.Uint256ParseFromBytes(porPackage.TxHash)
}

func (ps *PorServer) GetMiningSigChainTxn(txnHash common.Uint256) (*transaction.Transaction, error) {
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

func (ps *PorServer) AddSigChainFromTx(txn *transaction.Transaction, currentHeight uint32) (bool, error) {
	porPkg, err := NewPorPackage(txn)
	if err != nil {
		return false, err
	}

	voteForHeight := porPkg.GetVoteForHeight()
	if voteForHeight < currentHeight+2 {
		return false, fmt.Errorf("sigchain vote for height %d is less than %d", voteForHeight, currentHeight+2)
	}

	ps.Lock()
	defer ps.Unlock()

	if ps.miningPorPackage[voteForHeight] != nil && bytes.Compare(porPkg.SigHash, ps.miningPorPackage[voteForHeight].SigHash) >= 0 {
		return false, nil
	}

	err = ps.sigChainTxnCache.Add(porPkg.TxHash, txn)
	if err != nil {
		return false, err
	}

	ps.miningPorPackage[voteForHeight] = porPkg

	return true, nil
}

func (ps *PorServer) GetThreshold() common.Uint256 {
	//TODO get from block
	return common.Uint256{}
}

func (ps *PorServer) UpdateThreshold() common.Uint256 {
	//TODO used for new block
	return common.Uint256{}
}
