package por

import (
	"bytes"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
)

func TestPorPackage(t *testing.T) {
	crypto.SetAlg("P256R1")

	from, _ := vault.NewAccount()
	rel, _ := vault.NewAccount()
	to, _ := vault.NewAccount()
	toPk, _ := to.PubKey().EncodePoint(true)
	relPk, _ := rel.PubKey().EncodePoint(true)

	var srcID []byte
	dataHash := common.Uint256{}
	blockHash := common.Uint256{}
	sc, err := NewSigChain(from, 1, dataHash[:], blockHash[:], srcID, toPk, relPk)
	if err != nil {
		t.Error("sigchain created failed")
	}

	err = sc.Sign(toPk, rel)
	if err != nil || sc.Verify() != nil {
		t.Error("'rel' sign in error")
	}

	err = sc.Sign(toPk, to)
	if err != nil || sc.Verify() != nil {
		t.Error("'to' sign in error")
	}

	buf, err := proto.Marshal(sc)
	txn, err := transaction.NewCommitTransaction(buf, from.ProgramHash)
	if err != nil {
		log.Error("txn wrong", txn)
	}

	ppkg, err := NewPorPackage(txn)
	if err != nil {
		log.Error("Create por package error", err)
	}

	//test Hash
	ppkgHash := ppkg.SigHash
	sigChainHash, err := sc.SignatureHash()
	if bytes.Compare(ppkgHash, sigChainHash) != 0 {
		t.Error("[TestPorPackage] Hash test failed")
	}

	//GetBlockHash
	if bytes.Compare(sc.GetBlockHash(), ppkg.GetBlockHash()) != 0 {
		t.Error("[TestPorPackage] GetBlockHeight test failed")
	}

	//GetTxHash
	txHash := txn.Hash()
	if bytes.Compare(ppkg.GetTxHash(), txHash[:]) != 0 {
		t.Error("[TestPorPackage] GetTxHash test failed")
	}

	//GetSigChain
	sigChainHash, err = ppkg.GetSigChain().SignatureHash()
	if bytes.Compare(ppkgHash, sigChainHash) != 0 {
		t.Error("[TestPorPackage] GetSigChain test failed")
	}

	//Serialize & Deserialize
	buf, err = proto.Marshal(ppkg)
	ppkg2 := new(PorPackage)
	proto.Unmarshal(buf, ppkg2)

	if ppkg.CompareTo(ppkg2) != 0 {
		t.Error("[TestPorPackage] Serialize test failed")
	}
}
