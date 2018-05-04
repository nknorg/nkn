package por

import (
	"bytes"
	"testing"

	"github.com/ethereum/log"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/wallet"
)

func TestPorPackage(t *testing.T) {
	crypto.SetAlg("P256R1")
	from, _ := wallet.NewAccount()
	rel, _ := wallet.NewAccount()
	to, _ := wallet.NewAccount()
	pmFrom := NewPorServer(from)
	pmRel := NewPorServer(rel)
	pmTo := NewPorServer(to)
	toPk, _ := to.PubKey().EncodePoint(true)
	relPk, _ := rel.PubKey().EncodePoint(true)

	scFrom, err := pmFrom.CreateSigChain(1, &common.Uint256{}, toPk, relPk)
	if err != nil {
		t.Error("sigchain created failed")
	}
	scRel, _ := pmRel.Sign(scFrom, toPk)
	if pmRel.Verify(scRel) == nil {
		t.Log("[pormanager] verify successfully")
	} else {
		t.Error("[pormanager] verify failed")
	}

	scTo, _ := pmTo.Sign(scRel, toPk)
	if pmTo.Verify(scTo) == nil {
		t.Log("[pormanager] verify successfully 2")
	} else {
		t.Error("[pormanager] verify failed 2")
	}

	buff := bytes.NewBuffer(nil)
	scTo.Serialize(buff)
	txn, err := transaction.NewCommitTransaction(buff.Bytes())
	if err != nil {
		log.Error("txn wrong", txn)
	}

	ppkg := NewPorPackage(txn)
	ppkg.DumpInfo()
}
