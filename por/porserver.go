package por

import (
	"bytes"
	"errors"
	"sort"
	"sync"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
)

type PorServer struct {
	sync.RWMutex
	account    *vault.Account
	id         []byte
	pors       map[uint32][]*PorPackage
	minSigHash map[uint32][]byte
}

var porServer *PorServer

func NewPorServer(account *vault.Account, id []byte) *PorServer {
	ps := &PorServer{
		account:    account,
		id:         id,
		pors:       make(map[uint32][]*PorPackage),
		minSigHash: make(map[uint32][]byte),
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

//TODO subscriber
func (ps *PorServer) CleanChainList(height uint32) {

	ps.Lock()
	if height == 0 {
		ps.pors = make(map[uint32][]*PorPackage)
	} else {
		if _, ok := ps.pors[height]; ok {
			delete(ps.pors, height)
		}
	}
	ps.Unlock()
}

func (ps *PorServer) LenOfSigChain(sc *SigChain) int {
	return sc.Length()
}

func (ps *PorServer) GetMiningSigChain(height uint32) (*SigChain, error) {
	ps.RLock()
	defer ps.RUnlock()

	length := len(ps.pors[height])
	switch {
	case length > 1:
		sort.Sort(PorPackages(ps.pors[height]))
		fallthrough
	case length == 1:
		return ps.pors[height][0].GetSigChain(), nil
	default:
		return nil, nil
	}
}

func (ps *PorServer) GetMiningSigChainTxnHash(height uint32) (common.Uint256, error) {
	sigChain, err := ps.GetMiningSigChain(height)
	if err != nil || sigChain == nil {
		return common.EmptyUint256, err
	}

	sigHash, err := sigChain.SignatureHash()
	if err != nil {
		return common.EmptyUint256, err
	}

	txnHash := ps.GetTxnHashBySigChainHash(sigHash, height)
	if txnHash == common.EmptyUint256 {
		return common.EmptyUint256, errors.New("signature chain doesn't exist")
	}

	return txnHash, nil
}

func (ps *PorServer) GetSigChain(height uint32, hash common.Uint256) (*SigChain, error) {
	ps.RLock()
	defer ps.RUnlock()

	for _, pkg := range ps.pors[height] {
		if bytes.Compare(pkg.SigHash, hash[:]) == 0 {
			return pkg.SigChain, nil
		}
	}

	return nil, errors.New("can't find the signature chain")
}

func (ps *PorServer) GetTxnHashBySigChainHeight(height uint32) ([]common.Uint256, error) {
	ps.RLock()
	defer ps.RUnlock()

	var txnHashes []common.Uint256
	var porPackages []*PorPackage
	for h, p := range ps.pors {
		if h >= height {
			porPackages = append(porPackages, p...)
		}
	}
	for _, p := range porPackages {
		hash, err := common.Uint256ParseFromBytes(p.TxHash)
		if err != nil {
			return nil, errors.New("invalid transaction hash for por package")
		}
		txnHashes = append(txnHashes, hash)
	}

	return txnHashes, nil
}

func (ps *PorServer) AddSigChainFromTx(txn *transaction.Transaction) (bool, error) {
	porpkg, err := NewPorPackage(txn)
	if err != nil {
		return false, err
	}

	height := porpkg.GetVoteForHeight()
	ps.Lock()
	defer ps.Unlock()

	if ps.minSigHash[height] == nil || bytes.Compare(porpkg.SigHash, ps.minSigHash[height]) < 0 {
		ps.minSigHash[height] = porpkg.SigHash
	} else {
		return false, nil
	}

	if _, ok := ps.pors[height]; !ok {
		ps.pors[height] = make([]*PorPackage, 0)
	}
	ps.pors[height] = append(ps.pors[height], porpkg)

	return true, nil
}

func (ps *PorServer) GetTxnHashBySigChainHash(hash []byte, height uint32) common.Uint256 {
	ps.RLock()
	defer ps.RUnlock()
	for _, pkg := range ps.pors[height] {
		if bytes.Compare(pkg.SigHash, hash[:]) == 0 {
			txHash, err := common.Uint256ParseFromBytes(pkg.GetTxHash())
			if err != nil {
				log.Error(err)
				return common.EmptyUint256
			}
			return txHash
		}
	}
	return common.EmptyUint256
}

func (ps *PorServer) GetThreshold() common.Uint256 {
	//TODO get from block
	return common.Uint256{}
}

func (ps *PorServer) UpdateThreshold() common.Uint256 {
	//TODO used for new block
	return common.Uint256{}
}
