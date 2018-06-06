package voting

import (
	"errors"
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/por"
)

type SigChainVoting struct {
	sync.RWMutex
	pstate         map[Uint256]*State            // consensus state for proposer
	vstate         map[uint64]map[Uint256]*State // consensus state for voter
	height         uint32                        // voting height
	porServer      *por.PorServer                // signature chain service provider
	pool           *SigChainVotingPool           // signature chain voting pool
	confirmingHash Uint256                       // signature chain hash in process
	txnCollector   *transaction.TxnCollector     // transaction pool
}

func NewSigChainVoting(totalWeight int, txnCollector *transaction.TxnCollector) *SigChainVoting {
	sigChainVoting := &SigChainVoting{
		pstate:       make(map[Uint256]*State),
		vstate:       make(map[uint64]map[Uint256]*State),
		height:       ledger.DefaultLedger.Store.GetHeight() + 2,
		porServer:    por.GetPorServer(),
		pool:         NewSigChainVotingPool(totalWeight),
		txnCollector: txnCollector,
	}

	return sigChainVoting
}

func (scv *SigChainVoting) SetSelfState(hash Uint256, s State) {
	scv.Lock()
	defer scv.Unlock()
	if _, ok := scv.pstate[hash]; !ok {
		scv.pstate[hash] = new(State)
	}
	scv.pstate[hash].SetBit(s)
}

func (scv *SigChainVoting) HasSelfState(hash Uint256, state State) bool {
	scv.RLock()
	defer scv.RUnlock()

	if v, ok := scv.pstate[hash]; !ok || v == nil {
		return false
	} else {
		if v.HasBit(state) {
			return true
		}
		return false
	}
}

func (scv *SigChainVoting) SetNeighborState(id uint64, hash Uint256, s State) {
	scv.Lock()
	defer scv.Unlock()

	if _, ok := scv.vstate[id]; !ok {
		scv.vstate[id] = make(map[Uint256]*State)
	}
	if _, ok := scv.vstate[id][hash]; !ok {
		scv.vstate[id][hash] = new(State)
	}
	scv.vstate[id][hash].SetBit(s)
}

func (scv *SigChainVoting) HasNeighborState(id uint64, hash Uint256, state State) bool {
	scv.RLock()
	defer scv.RUnlock()

	if _, ok := scv.vstate[id]; !ok {
		return false
	} else {
		if v, ok := scv.vstate[id][hash]; !ok || v == nil {
			return false
		} else {
			if v.HasBit(state) {
				return true
			}
			return false
		}
	}
}

func (scv *SigChainVoting) SetVotingHeight(height uint32) {
	scv.height = height
}

func (scv *SigChainVoting) UpdateVotingHeight() {
	scv.height = ledger.DefaultLedger.Store.GetHeight() + 2
}

func (scv *SigChainVoting) GetVotingHeight() uint32 {
	return scv.height
}

func (scv *SigChainVoting) SetConfirmingHash(hash Uint256) {
	scv.confirmingHash = hash
}

func (scv *SigChainVoting) GetConfirmingHash() Uint256 {
	return scv.confirmingHash
}

func (scv *SigChainVoting) GetBestVotingContent(height uint32) (VotingContent, error) {
	sigChain, err := scv.porServer.GetMinSigChain(height)
	if err != nil {
		return nil, err
	}
	sigHash, err := sigChain.SignatureHash()
	if err != nil {
		return nil, err
	}
	txnHash, exist := scv.porServer.IsSigChainExist(sigHash, height)
	if !exist {
		return nil, errors.New("signature chain doesn't exist")
	}
	txnInPool := scv.txnCollector.GetTransaction(*txnHash)
	if txnInPool != nil {
		return txnInPool, nil
	}
	txnInLedger, err := ledger.DefaultLedger.Store.GetTransaction(*txnHash)
	if err == nil {
		return txnInLedger, nil
	}

	return nil, errors.New("invalid commit transaction")

}

func (scv *SigChainVoting) GetWorseVotingContent(height uint32) (VotingContent, error) {
	return nil, nil
}

func (scv *SigChainVoting) GetVotingContentFromPool(hash Uint256, height uint32) (VotingContent, error) {
	sigChain, err := scv.porServer.GetSigChain(height, hash)
	if err != nil {
		return nil, err
	}
	sigHash, err := sigChain.SignatureHash()
	if err != nil {
		return nil, err
	}
	txnHash, exist := scv.porServer.IsSigChainExist(sigHash, height)
	if !exist {
		return nil, errors.New("signature chain doesn't exist")
	}
	txn := scv.txnCollector.GetTransaction(*txnHash)
	if txn == nil {
		return nil, errors.New("invalid hash for transaction")
	}

	return txn, nil
}

func (scv *SigChainVoting) GetVotingContent(hash Uint256, height uint32) (VotingContent, error) {
	// get signature chain by height and hash
	sigChain, err := scv.porServer.GetSigChain(height, hash)
	if err != nil {
		return nil, err
	}
	sigHash, err := sigChain.SignatureHash()
	if err != nil {
		return nil, err
	}
	// get transaction hash by signature chain
	txnHash, exist := scv.porServer.IsSigChainExist(sigHash, height)
	if !exist {
		return nil, errors.New("signature chain doesn't exist")
	}
	// get transaction from transaction pool
	txnInPool := scv.txnCollector.GetTransaction(*txnHash)
	if txnInPool != nil {
		return txnInPool, nil
	}
	// get transaction from ledger
	txnInLedger, err := ledger.DefaultLedger.Store.GetTransaction(*txnHash)
	if err == nil {
		return txnInLedger, nil
	}

	return nil, errors.New("invalid commit transaction")
}

func (scv *SigChainVoting) VotingType() VotingContentType {
	return SigChainTxnVote
}

func (scv *SigChainVoting) Preparing(content VotingContent) error {
	errCode := scv.txnCollector.Append(content.(*transaction.Transaction))
	if errCode != 0 {
		return errors.New("append transaction error")
	}

	return nil
}

func (scv *SigChainVoting) Exist(hash Uint256, height uint32) bool {
	ret := scv.txnCollector.GetTransaction(hash)
	_, err := ledger.DefaultLedger.Store.GetTransaction(hash)
	// return false if the Commit transaction doesn't exist in transaction pool and ledger
	// TODO: need to check if the err returned from DB is NOTFOUND
	if ret == nil && err != nil {
		return false
	}

	return true
}

func (scv *SigChainVoting) GetVotingPool() VotingPool {
	return scv.pool
}

func (scv *SigChainVoting) DumpState(hash Uint256, desc string, verbose bool) {
}
