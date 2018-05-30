package por

import (
	"errors"
	"sort"
	"sync"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
)

type PorServer struct {
	sync.RWMutex
	account *wallet.Account
	pors    map[uint32][]*porPackage
}

var porServer *PorServer

func NewPorServer(account *wallet.Account) *PorServer {
	ps := &PorServer{
		account: account,
		pors:    make(map[uint32][]*porPackage),
	}
	return ps
}

func InitPorServer(account *wallet.Account) error {
	if porServer != nil {
		return errors.New("PorServer already initialized")
	}
	porServer = NewPorServer(account)
	return nil
}

func GetPorServer() *PorServer {
	if porServer == nil {
		panic("PorServer not initialized")
	}
	return porServer
}

func (ps *PorServer) Sign(sc *SigChain, nextPubkey []byte) error {
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

	err = sc.Sign(nextPubkey, ps.account)
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

func (ps *PorServer) CreateSigChain(height, dataSize uint32, dataHash *common.Uint256, destPubkey, nextPubkey []byte) (*SigChain, error) {
	return NewSigChain(ps.account, height, dataSize, dataHash, destPubkey, nextPubkey)
}

func (ps *PorServer) CreateSigChainForClient(height, dataSize uint32, dataHash *common.Uint256, srcPubkey, destPubkey, nextPubkey, signature []byte) (*SigChain, error) {
	pubKey, err := ps.account.PubKey().EncodePoint(true)
	if err != nil {
		log.Error("Get account public key error:", err)
		return nil, err
	}
	sigChain, err := NewSigChainWithSignature(height, dataSize, dataHash, srcPubkey, destPubkey, pubKey, signature)
	if err != nil {
		log.Error("New signature chain with signature error:", err)
		return nil, err
	}
	err = ps.Sign(sigChain, nextPubkey)
	if err != nil {
		log.Error("Signing signature chain error:", err)
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
		ps.pors = make(map[uint32][]*porPackage)
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

func (ps *PorServer) GetMinSigChain(height uint32) (*SigChain, error) {
	ps.RLock()
	defer ps.RUnlock()

	var minSigChain *SigChain
	length := len(ps.pors[height])
	switch {
	case length > 1:
		sort.Sort(porPackages(ps.pors[height]))
		fallthrough
	case length == 1:
		minSigChain = ps.pors[height][0].GetSigChain()
		return minSigChain, nil
	case length < 1:
		return nil, errors.New("no available signature chain")
	}

	return minSigChain, nil
}

func (ps *PorServer) GetSigChain(height uint32, hash common.Uint256) (*SigChain, error) {
	ps.RLock()
	defer ps.RUnlock()

	for _, pkg := range ps.pors[height] {
		pkgHash := pkg.Hash()
		if hash.CompareTo(pkgHash) == 0 {
			return pkg.sigchain, nil
		}
	}

	return nil, errors.New("can't find the signature chain")
}

func (ps *PorServer) AddSigChainFromTx(txn *transaction.Transaction) error {
	porpkg := NewPorPackage(txn)
	if porpkg == nil {
		return errors.New("the type of transaction is mismatched")
	}

	height := porpkg.GetHeight()
	ps.Lock()
	if _, ok := ps.pors[height]; !ok {
		ps.pors[height] = make([]*porPackage, 0)
	}
	ps.pors[height] = append(ps.pors[height], porpkg)
	ps.Unlock()

	return nil
}

func (ps *PorServer) IsSigChainExist(hash common.Uint256, height uint32) (*common.Uint256, bool) {
	ps.RLock()
	for _, pkg := range ps.pors[height] {
		pkgHash := pkg.Hash()
		if (&pkgHash).CompareTo(hash) == 0 {
			ps.RUnlock()
			return pkg.GetTxHash(), true
		}
	}
	ps.RUnlock()
	return nil, false
}

func (ps *PorServer) GetThreshold() common.Uint256 {
	//TODO get from block
	return common.Uint256{}
}

func (ps *PorServer) UpdateThreshold() common.Uint256 {
	//TODO used for new block
	return common.Uint256{}
}
