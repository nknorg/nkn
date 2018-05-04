package por

import (
	"errors"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/wallet"
)

const (
	msgChanCap = 10
)

type porServer struct {
	started   bool
	account   *wallet.Account
	pors      map[uint32][]*porPackage
	msgChan   chan interface{}
	relayChan chan chan *porPackage
	quit      chan struct{}
}

func NewPorServer(account *wallet.Account) *porServer {
	ps := &porServer{
		account:   account,
		msgChan:   make(chan interface{}, msgChanCap),
		relayChan: make(chan chan *porPackage),
		pors:      make(map[uint32][]*porPackage),
		quit:      make(chan struct{}, 1),
	}

	go ps.porHandler()
	ps.started = true
	return ps
}

func (p *porServer) porHandler() {
out:
	for {
		select {
		case m := <-p.msgChan:
			switch msg := m.(type) {
			case *porPackage:
				height := msg.GetHeight()
				if _, ok := p.pors[height]; !ok {
					p.pors[height] = make([]*porPackage, 0)
				}
				p.pors[height] = append(p.pors[height], msg)
			case uint32:
				if msg == 0 {
					p.pors = make(map[uint32][]*porPackage)
				} else {
					if _, ok := p.pors[msg]; ok {
						delete(p.pors, msg)
					}
				}
			}
		case msg := <-p.relayChan:
			//TODO get minimum sigchain
			msg <- &porPackage{}
		case <-p.quit:
			break out
		}

	}
}

func (p *porServer) Stop() {
	p.quit <- struct{}{}
	p.started = false
}

func (p *porServer) Sign(sc *SigChain, nextPubkey []byte) (*SigChain, error) {
	dcPk, err := p.account.PubKey().EncodePoint(true)
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

	err = sc.Sign(nextPubkey, p.account)
	if err != nil {
		return nil, errors.New("sign failed")
	}

	if sc.IsFinal() {
		//TODO send commit tx
		//TODO create porPackage
		p.msgChan <- porPackage{}
	}

	return sc, nil
}

func (p *porServer) Verify(sc *SigChain) error {
	if err := sc.Verify(); err != nil {
		return errors.New("verify failed")
	}

	return nil
}

func (p *porServer) CreateSigChain(dataSize uint32, dataHash *common.Uint256, destPubkey []byte, nextPubkey []byte) (*SigChain, error) {
	return NewSigChain(p.account, dataSize, dataHash, destPubkey, nextPubkey)
}

func (p *porServer) IsFinal(sc *SigChain) bool {
	return sc.IsFinal()
}

func (p *porServer) GetSignture(sc *SigChain) ([]byte, error) {
	return sc.GetSignture()
}

//TODO subscripter
func (p *porServer) CleanChainList(height uint32) {
	if !p.started {
		return
	}

	p.msgChan <- height
}

func (p *porServer) LenOfSigChain(sc *SigChain) int {
	return sc.Length()
}

func (p *porServer) GetMinSigChain() *SigChain {
	if !p.started {
		return nil
	}

	pr := make(chan *porPackage, 1)
	p.relayChan <- pr
	minPor := <-pr

	return minPor.GetSigChain()
}

func (p *porServer) AddSigChainFromTx(txn transaction.Transaction) {
	porpkg := NewPorPackage(txn)

	p.msgChan <- porpkg
}

func (p *porServer) GetThreshold() common.Uint256 {
	//TODO get from block
	return []common.Uint256{}
}

func (p *porServer) UpdateThreshold() common.Uint256 {
	//TODO used for new block
	return []common.Uint256{}
}
