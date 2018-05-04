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

func (ps *porServer) porHandler() {
out:
	for {
		select {
		case m := <-ps.msgChan:
			switch msg := m.(type) {
			case *porPackage:
				height := msg.GetHeight()
				if _, ok := ps.pors[height]; !ok {
					ps.pors[height] = make([]*porPackage, 0)
				}
				ps.pors[height] = append(ps.pors[height], msg)
			case uint32:
				if msg == 0 {
					ps.pors = make(map[uint32][]*porPackage)
				} else {
					if _, ok := ps.pors[msg]; ok {
						delete(ps.pors, msg)
					}
				}
			}
		case msg := <-ps.relayChan:
			//TODO get minimum sigchain
			msg <- &porPackage{}
		case <-ps.quit:
			break out
		}

	}
}

func (ps *porServer) Stop() {
	ps.quit <- struct{}{}
	ps.started = false
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

	if sc.IsFinal() {
		//TODO send commit tx
		//TODO create porPackage
		ps.msgChan <- porPackage{}
	}

	return sc, nil
}

func (ps *porServer) Verify(sc *SigChain) error {
	if err := sc.Verify(); err != nil {
		return errors.New("verify failed")
	}

	return nil
}

func (ps *porServer) CreateSigChain(dataSize uint32, dataHash *common.Uint256, destPubkey []byte, nextPubkey []byte) (*SigChain, error) {
	return NewSigChain(ps.account, dataSize, dataHash, destPubkey, nextPubkey)
}

func (ps *porServer) IsFinal(sc *SigChain) bool {
	return sc.IsFinal()
}

func (ps *porServer) GetSignture(sc *SigChain) ([]byte, error) {
	return sc.GetSignture()
}

//TODO subscripter
func (ps *porServer) CleanChainList(height uint32) {
	if !ps.started {
		return
	}

	ps.msgChan <- height
}

func (ps *porServer) LenOfSigChain(sc *SigChain) int {
	return sc.Length()
}

func (ps *porServer) GetMinSigChain() *SigChain {
	if !ps.started {
	}

	pr := make(chan *porPackage, 1)
	ps.relayChan <- pr
	minPor := <-pr

	return minPor.GetSigChain()
}

func (ps *porServer) AddSigChainFromTx(txn *transaction.Transaction) {
	porpkg := NewPorPackage(txn)

	ps.msgChan <- porpkg
}

func (ps *porServer) GetThreshold() common.Uint256 {
	//TODO get from block
	return common.Uint256{}
}

func (ps *porServer) UpdateThreshold() common.Uint256 {
	//TODO used for new block
	return common.Uint256{}
}
