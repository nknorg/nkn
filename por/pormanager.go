package por

import (
	"bytes"
	"crypto/sha256"
	"errors"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
)

const (
	cacheChanCap = 10
)

type porManager struct {
	account    *wallet.Account
	maxWorkSig []byte
	quit       chan struct{}
	started    bool
	sigChains  map[common.Uint256]*SigChain
	cacheChan  chan interface{}
}

type getSigChainsMsg struct {
	reply chan []*SigChain
}

func NewPorManager(acc *wallet.Account) *porManager {
	pm := &porManager{
		account:   acc,
		cacheChan: make(chan interface{}, cacheChanCap),
		quit:      make(chan struct{}, 1),
		sigChains: make(map[common.Uint256]*SigChain),
	}

	go pm.cacheSigChain()
	pm.started = true
	return pm
}

func (pm *porManager) cacheSigChain() {
out:
	for {
		select {
		case m := <-pm.cacheChan:
			switch msg := m.(type) {
			case *SigChain:
				buff := bytes.NewBuffer(nil)
				if err := msg.Serialize(buff); err != nil {
					log.Error("sigchain Serialize error")
				} else {
					hash := sha256.Sum256(buff.Bytes())
					pm.sigChains[hash] = msg
				}
			case []common.Uint256:
				for _, v := range msg {
					_, ok := pm.sigChains[v]
					if ok {
						delete(pm.sigChains, v)
					}
				}
			case *getSigChainsMsg:
				sigchains := make([]*SigChain, 0, len(pm.sigChains))
				for _, sigchain := range pm.sigChains {
					sigchains = append(sigchains, sigchain)
				}
				msg.reply <- sigchains
			}
		case <-pm.quit:
			break out
		}

	}
}

func (pm *porManager) Sign(sc *SigChain, nextPubkey []byte) (*SigChain, error) {
	dcPk, err := pm.account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, errors.New("the account of porManager is wrong")
	}

	nxPk, err := sc.GetLastPubkey()
	if err != nil {
		return nil, errors.New("can't get nexpubkey")
	}

	if !common.IsEqualBytes(dcPk, nxPk) {
		return nil, errors.New("it's not the right signer")
	}

	err = sc.Sign(nextPubkey, pm.account)
	if err != nil {
		return nil, errors.New("sign failed")
	}

	if sc.IsFinal() {
		pm.cacheChan <- sc
	}

	return sc, nil
}

func (pm *porManager) Verify(sc *SigChain) error {
	if err := sc.Verify(); err != nil {
		return errors.New("verify failed")
	}

	return nil
}

func (pm *porManager) CreateSigChain(dataSize uint32, dataHash *common.Uint256, destPubkey []byte, nextPubkey []byte) (*SigChain, error) {
	return NewSigChain(pm.account, dataSize, dataHash, destPubkey, nextPubkey)
}

func (pm *porManager) IsFinal(sc *SigChain) bool {
	return sc.IsFinal()
}

func (pm *porManager) GetSignture(sc *SigChain) ([]byte, error) {
	return sc.GetSignture()
}

func (pm *porManager) CleanChainCache(sigchainHashs []common.Uint256) {
	if !pm.started {
		return
	}

	pm.cacheChan <- sigchainHashs
}

func (pm *porManager) LenOfSigChain(sc *SigChain) int {
	return sc.Length()
}

func (pm *porManager) GetSigChains() []*SigChain {
	if !pm.started {
		return nil
	}

	rp := make(chan []*SigChain, 1)
	pm.cacheChan <- &getSigChainsMsg{reply: rp}
	ret := <-rp
	return ret
}
