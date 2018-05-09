package por

import (
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
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
	GetHeight() uint32
	GetSignature() ([]byte, error)
	GetOwner() ([]byte, error)
	Serialize(w io.Writer) error
	Deserialize(r io.Reader) error
	Hash() common.Uint256
}

type SigChain struct {
	sigType string
	chain   sigchainer
}

func NewSigChain(account *wallet.Account, height, dataSize uint32, dataHash *common.Uint256, destPubkey, nextPubkey []byte) (*SigChain, error) {
	var sigchain sigchainer
	var sigtype string
	switch sigChainAlg {
	case Ed25519:
	case ECDSAP256R1:
		fallthrough
	default:
		sigtype = ECDSAP256R1
		sigchain, _ = sigchains.NewSigChainEcdsa(account, height, dataSize, dataHash, destPubkey, nextPubkey)
	}

	return &SigChain{sigtype, sigchain}, nil
}

func (sc *SigChain) Sign(nextPubkey []byte, signer *wallet.Account) error {
	return sc.chain.Sign(nextPubkey, signer)
}

func (sc *SigChain) Verify() error {
	return sc.chain.Verify()
}

func (sc *SigChain) Path() [][]byte {
	return sc.chain.Path()
}

func (sc *SigChain) Length() int {
	return sc.chain.Length()
}

func (sc *SigChain) IsFinal() bool {
	return sc.chain.IsFinal()
}

func (sc *SigChain) GetSignerIndex(pubkey []byte) (int, error) {
	return sc.chain.GetSignerIndex(pubkey)
}

func (sc *SigChain) GetDataHash() *common.Uint256 {
	return sc.chain.GetDataHash()
}

func (sc *SigChain) Serialize(w io.Writer) error {
	if err := serialization.WriteVarString(w, sc.sigType); err != nil {
		return err
	}

	if err := sc.chain.Serialize(w); err != nil {
		return err
	}
	return nil
}

func (sc *SigChain) Deserialize(r io.Reader) error {
	var err error
	sc.sigType, err = serialization.ReadVarString(r)
	if err != nil {
		return err
	}

	switch sc.sigType {
	case ECDSAP256R1:
		sc.chain = new(sigchains.SigChainEcdsa)
		if err := sc.chain.Deserialize(r); err != nil {
			return err
		}
	}

	return nil
}

func (sc *SigChain) GetLastPubkey() ([]byte, error) {
	return sc.chain.GetLastPubkey()
}

func (sc *SigChain) GetSignature() ([]byte, error) {
	return sc.chain.GetSignature()
}

func (sc *SigChain) Hash() common.Uint256 {
	return sc.chain.Hash()
}

func (sc *SigChain) GetHeight() uint32 {
	return sc.chain.GetHeight()
}

func (sc *SigChain) GetOwner() ([]byte, error) {
	return sc.chain.GetOwner()
}
