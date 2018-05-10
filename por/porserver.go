package por

import (
	"errors"
	"sort"
	"sync"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/wallet"
)

type porServer struct {
	sync.RWMutex
	account *wallet.Account
	pors    map[uint32][]*porPackage
}

func NewPorServer(account *wallet.Account) *porServer {
	ps := &porServer{
		account: account,
		pors:    make(map[uint32][]*porPackage),
	}

	return ps
}

type porPackages []*porPackage

func (c porPackages) Len() int {
	return len(c)
}
func (c porPackages) Swap(i, j int) {
	if i >= 0 && i < len(c) && j >= 0 && j < len(c) { // Unit Test modify
		c[i], c[j] = c[j], c[i]
	}
}
func (c porPackages) Less(i, j int) bool {
	if i >= 0 && i < len(c) && j >= 0 && j < len(c) { // Unit Test modify
		return c[i].CompareTo(c[j]) < 0
	}

	return false
}

func (ps *porServer) Sign(sc *SigChain, nextPubkey []byte) (*SigChain, error) {
	dcPk, err := ps.account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, errors.New("the account of porServer is wrong")
	}

	nxPk, err := sc.GetLastPubkey()
	if err != nil {
		return nil, errors.New("can't get nexpubkey")
	}

	if !common.IsEqualBytes(dcPk, nxPk) {
		return nil, errors.New("it's not the right signer")
	}

	err = sc.Sign(nextPubkey, ps.account)
	if err != nil {
		return nil, errors.New("sign failed")
	}

	return sc, nil
}

func (ps *porServer) Verify(sc *SigChain) error {
	if err := sc.Verify(); err != nil {
		return errors.New("verify failed")
	}

	return nil
}

func (ps *porServer) CreateSigChain(height, dataSize uint32, dataHash *common.Uint256, destPubkey, nextPubkey []byte) (*SigChain, error) {
	return NewSigChain(ps.account, height, dataSize, dataHash, destPubkey, nextPubkey)
}

func (ps *porServer) IsFinal(sc *SigChain) bool {
	return sc.IsFinal()
}

func (ps *porServer) GetSignature(sc *SigChain) ([]byte, error) {
	return sc.GetSignature()
}

//TODO subscriber
func (ps *porServer) CleanChainList(height uint32) {

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

func (ps *porServer) LenOfSigChain(sc *SigChain) int {
	return sc.Length()
}

func (ps *porServer) GetMinSigChain() *SigChain {
	height := ledger.DefaultLedger.Store.GetHeight()
	ps.RLock()
	sort.Sort(porPackages(ps.pors[height]))
	min := ps.pors[height][0]
	ps.RUnlock()

	return min.GetSigChain()
}

func (ps *porServer) AddSigChainFromTx(txn *transaction.Transaction) error {
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

func (ps *porServer) GetThreshold() common.Uint256 {
	//TODO get from block
	return common.Uint256{}
}

func (ps *porServer) UpdateThreshold() common.Uint256 {
	//TODO used for new block
	return common.Uint256{}
}
