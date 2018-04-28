package por

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
)

const (
	porChanCap   = 5
	cacheChanCap = 10
)

type porManager struct {
	account    *wallet.Account
	maxWorkSig []byte
	msgChan    chan interface{}
	quit       chan struct{}
	started    bool
	sigChains  map[common.Uint256]*SigChain
	cacheChan  chan interface{}
}

type signMsg struct {
	sc         *SigChain
	nextPubkey *crypto.PubKey
	reply      chan *SigChain
}

type verifyMsg struct {
	sc    *SigChain
	reply chan bool
}
type getSigChainsMsg struct {
	reply chan []*SigChain
}

func NewPorManager(acc *wallet.Account) *porManager {
	pm := &porManager{
		account:   acc,
		msgChan:   make(chan interface{}, porChanCap),
		cacheChan: make(chan interface{}, cacheChanCap),
		quit:      make(chan struct{}, 1),
		sigChains: make(map[common.Uint256]*SigChain),
	}

	go pm.porHandler()
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

func (pm *porManager) porHandler() {
out:
	for {
		select {
		case m := <-pm.msgChan:
			switch msg := m.(type) {
			case *signMsg:
				go pm.handleSignMsg(msg)
			case *verifyMsg:
				go pm.handleVerifyMsg(msg)
			default:
				log.Warnf("Invalid message type in block "+"handler: %T", msg)
			}

		case <-pm.quit:
			break out
		}
	}

	log.Trace("Por handler done")
}

func (pm *porManager) handleSignMsg(sm *signMsg) {
	if !crypto.Equal(pm.account.PubKey(), sm.sc.elems[len(sm.sc.elems)-1].nextPubkey) {
		sm.reply <- sm.sc
	}
	err := sm.sc.Sign(sm.nextPubkey, pm.account)
	if err != nil {
		fmt.Println(err)
	}
	sm.reply <- sm.sc

	if sm.sc.IsFinal() {
		pm.cacheChan <- sm.sc
	}
}

func (pm *porManager) handleVerifyMsg(sm *verifyMsg) {
	if err := sm.sc.Verify(); err != nil {
		sm.reply <- false
	}

	sm.reply <- true
}

func (pm *porManager) Sign(sc *SigChain, nextPubkey *crypto.PubKey) *SigChain {
	if !pm.started {
		return nil
	}

	rp := make(chan *SigChain, 1)
	pm.msgChan <- &signMsg{sc: sc, nextPubkey: nextPubkey, reply: rp}
	ret := <-rp
	return ret
}

func (pm *porManager) Verify(sc *SigChain) bool {
	if !pm.started {
		return false
	}

	rp := make(chan bool, 1)
	pm.msgChan <- &verifyMsg{sc: sc, reply: rp}
	ret := <-rp
	return ret
}

func (pm *porManager) CreateSigChain(dataSize uint32, dataHash *common.Uint256, destPubkey *crypto.PubKey, nextPubkey *crypto.PubKey) (*SigChain, error) {
	return NewSigChain(pm.account, dataSize, dataHash, destPubkey, nextPubkey)
}

func (pm *porManager) IsFinal(sc *SigChain) bool {
	return sc.IsFinal()
}

func (pm *porManager) GetSignture(sc *SigChain) ([]byte, error) {
	sce, err := sc.finalSigElem()
	if err != nil {
		return nil, err
	}

	return sce.signature, nil
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
	pm.msgChan <- &getSigChainsMsg{reply: rp}
	ret := <-rp
	return ret
}
