package por

import (
	"bytes"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/por/sigchains"
	"github.com/nknorg/nkn/util/log"
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

	//TODO threshold

	owner, _ := sigchain.GetOwner()
	return &porPackage{
		owner:        owner,
		height:       sigchain.GetHeight(),
		txHash:       txn.Hash(),
		sigchainHash: sigchain.Hash(),
		sigchain:     &sigchain,
	}
}

func (pp *porPackage) Hash() common.Uint256 {
	return pp.sigchainHash
}

func (pp *porPackage) GetHeight() uint32 {
	return pp.height
}

func (pp *porPackage) GetTxHash() *common.Uint256 {
	return &pp.txHash
}

func (pp *porPackage) GetSigChain() *SigChain {
	return pp.sigchain
}

func (pp *porPackage) CompareTo(o *porPackage) int {
	return (&pp.sigchainHash).CompareTo(o.sigchainHash)
}

func (pp *porPackage) Serialize(w io.Writer) error {
	var err error

	err = serialization.WriteVarBytes(w, pp.owner)
	if err != nil {
		return err
	}

	err = serialization.WriteUint32(w, pp.height)
	if err != nil {
		return err
	}

	_, err = pp.txHash.Serialize(w)
	if err != nil {
		return err
	}

	_, err = pp.sigchainHash.Serialize(w)
	if err != nil {
		return err
	}

	err = pp.sigchain.Serialize(w)

	return err
}

func (pp *porPackage) Deserialize(r io.Reader) error {
	var err error

	pp.owner, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	pp.height, err = serialization.ReadUint32(r)
	if err != nil {
		return err
	}

	err = pp.txHash.Deserialize(r)
	if err != nil {
		return err
	}

	err = pp.sigchainHash.Deserialize(r)
	if err != nil {
		return err
	}

	sigchain := new(SigChain)
	err = sigchain.Deserialize(r)
	if err != nil {
		return nil
	}
	pp.sigchain = sigchain

	return nil
}

func (pp *porPackage) DumpInfo() {
	log.Info("owner: ", common.BytesToHexString(pp.owner))
	log.Info("txHash: ", pp.txHash)
	log.Info("sigchainHash: ", pp.sigchainHash)
	if sc, ok := pp.sigchain.chain.(*sigchains.SigChainEcdsa); ok {
		sc.DumpInfo()
	}
}
