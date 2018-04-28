package por

import (
	"fmt"

	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
)

const (
	PorChanCap = 5
)

//TODO
//  Version() uint32
//  Sign(sc *SigChain) error
//  Verify(sc *SigChain) error
//  IsFinal(sc *SigChain) bool
//  GetSignture(sc *SigChain) ([]byte, error)
//  GetLenofSig(sc *SigChain) uint32
//  CommitSig() *transaction.Transaction
//  GetSigFromTx(txn *transaction.Transaction) []byte
//  GetSigChainFromTx(txn *transaction.Transaction) *SigChain
type porManager struct {
	account    *wallet.Account
	maxWorkSig []byte
	msgChan    chan interface{}
	quit       chan struct{}
	started    bool
}

func NewPorManager(acc *wallet.Account) *porManager {
	pm := &porManager{
		account: acc,
		msgChan: make(chan interface{}, PorChanCap),
		quit:    make(chan struct{}, 1),
	}

	go pm.porHandler()
	pm.started = true
	return pm
}

func (pm *porManager) porHandler() {
out:
	for {
		select {
		case m := <-pm.msgChan:
			switch msg := m.(type) {
			case *signMsg:
				go pm.handleSignMsg(msg)
			default:
				log.Warnf("Invalid message type in block "+"handler: %T", msg)
			}

		case <-pm.quit:
			break out
		}
	}

	log.Trace("Por handler done")
}

type signMsg struct {
	sc         *SigChain
	nextPubkey *crypto.PubKey
	reply      chan *SigChain
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
	//TODO if it's the destination address of sigchain, send commit
	//transaction
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
