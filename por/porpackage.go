package por

import (
	"bytes"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
)

type porPackage struct {
	owner        []byte
	height       uint32
	txHash       common.Uint256
	sigchainHash common.Uint256
	sigchain     *SigChain
}

func NewPorPackage(txn *transaction.Transaction) *porPackage {
	if txn.TxType != transaction.Commit {
		return nil
	}
	rs := txn.Payload.(*payload.Commit)
	buf := bytes.NewBuffer(rs.SigChain)
	var sigchain SigChain
	sigchain.Deserialize(buf)

	//TODO thershold

	owner, _ := sigchain.GetOwner()
	return &porPackage{
		owner:        owner,
		height:       sigchain.GetCurrentHeight(),
		txHash:       txn.Hash(),
		sigchainHash: sigchain.Hash(),
		sigchain:     &sigchain,
	}
}

func (p *porPackage) Hash() common.Uint256 {
	return p.sigchainHash
}

func (p *porPackage) GetHeight() uint32 {
	return p.height
}

func (p *porPackage) GetTxHash() *common.Uint256 {
	return &p.txHash
}

func (p *porPackage) GetSigChain() *SigChain {
	return p.sigchain
}

func (p *porPackage) CompareTo(o *porPackage) int {
	return (&p.sigchainHash).CompareTo(o.sigchainHash)
}

func (p *porPackage) Serialize(w io.Writer) error {
	var err error

	err = serialization.WriteVarBytes(w, p.owner)
	if err != nil {
		return err
	}

	err = serialization.WriteUint32(w, p.height)
	if err != nil {
		return err
	}

	_, err = p.txHash.Serialize(w)
	if err != nil {
		return err
	}

	_, err = p.sigchainHash.Serialize(w)
	if err != nil {
		return err
	}

	err = p.sigchain.Serialize(w)

	return err
}

func (p *porPackage) Deserialize(r io.Reader) error {
	var err error

	p.owner, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	p.height, err = serialization.ReadUint32(r)
	if err != nil {
		return err
	}

	err = p.txHash.Deserialize(r)
	if err != nil {
		return err
	}

	err = p.sigchainHash.Deserialize(r)
	if err != nil {
		return err
	}

	err = p.sigchain.Deserialize(r)

	return err
}
