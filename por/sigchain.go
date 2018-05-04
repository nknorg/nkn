package por

import (
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/por/sigchains"
	"github.com/nknorg/nkn/wallet"
)

var (
	//TODO sigChainAlg add to config.json
	sigChainAlg = "ECDSAP256R1"
	ECDSAP256R1 = "ECDSAP256R1"
	Ed25519     = "ED25519"
)

type sigchainer interface {
	Sign(nextPubkey []byte, signer *wallet.Account) error
	Verify() error
	Path() [][]byte
	Length() int
	IsFinal() bool
	GetSignerIndex(pubkey []byte) (int, error)
	GetLastPubkey() ([]byte, error)
	GetDataHash() *common.Uint256
	GetCurrentHeight() uint32
	GetSignture() ([]byte, error)
	GetOwner() ([]byte, error)
	Serialize(w io.Writer) error
	Deserialize(r io.Reader) error
	Hash() common.Uint256
}

type SigChain struct {
	chain sigchainer
}

func NewSigChain(owner *wallet.Account, dataSize uint32, dataHash *common.Uint256, destPubkey []byte, nextPubkey []byte) (*SigChain, error) {
	var sigchain sigchainer
	switch sigChainAlg {
	case Ed25519:
	case ECDSAP256R1:
		fallthrough
	default:
		sigchain, _ = sigchains.NewSigChainEcdsa(owner, dataSize, dataHash, destPubkey, nextPubkey)
	}

	return &SigChain{sigchain}, nil
}

func (p *SigChain) Sign(nextPubkey []byte, signer *wallet.Account) error {
	return p.chain.Sign(nextPubkey, signer)
}

func (p *SigChain) Verify() error {
	return p.chain.Verify()
}

func (p *SigChain) Path() [][]byte {
	return p.chain.Path()
}

func (p *SigChain) Length() int {
	return p.chain.Length()
}

func (p *SigChain) IsFinal() bool {
	return p.chain.IsFinal()
}

func (p *SigChain) GetSignerIndex(pubkey []byte) (int, error) {
	return p.chain.GetSignerIndex(pubkey)
}

func (p *SigChain) GetDataHash() *common.Uint256 {
	return p.chain.GetDataHash()
}

func (p *SigChain) Serialize(w io.Writer) error {
	if err := p.chain.Serialize(w); err != nil {
		return err
	}
	return nil
}

func (p *SigChain) Deserialize(r io.Reader) error {
	if err := p.chain.Deserialize(r); err != nil {
		return err
	}
	return nil
}

func (p *SigChain) GetLastPubkey() ([]byte, error) {
	return p.chain.GetLastPubkey()
}

func (p *SigChain) GetSignture() ([]byte, error) {
	return p.chain.GetLastPubkey()
}

func (p *SigChain) Hash() common.Uint256 {
	return p.chain.Hash()
}

func (p *SigChain) GetCurrentHeight() uint32 {
	return p.chain.GetCurrentHeight()
}

func (p *SigChain) GetOwner() ([]byte, error) {
	return p.chain.GetOwner()
}
