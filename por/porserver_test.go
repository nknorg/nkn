package por

import (
	"testing"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/wallet"
)

func TestPorServer(t *testing.T) {
	crypto.SetAlg("P256R1")
	from, _ := wallet.NewAccount()
	rel, _ := wallet.NewAccount()
	to, _ := wallet.NewAccount()
	pmFrom := NewPorServer(from)
	pmRel := NewPorServer(rel)
	pmTo := NewPorServer(to)
	toPk, _ := to.PubKey().EncodePoint(true)
	relPk, _ := rel.PubKey().EncodePoint(true)

	scFrom, err := pmFrom.CreateSigChain(1, &common.Uint256{}, &common.Uint256{}, toPk, relPk)
	if err != nil {
		t.Error("sigchain created failed")
	}
	pmRel.Sign(scFrom, toPk)
	if pmRel.Verify(scFrom) == nil {
		t.Log("[pormanager] verify successfully")
	} else {
		t.Error("[pormanager] verify failed")
	}

	if !pmTo.IsFinal(scFrom) {
		t.Log("[pormanager] IsFinal test successfully")
	} else {
		t.Error("[pormanager] IsFinal test failed")
	}

	pmTo.Sign(scFrom, toPk)
	if pmTo.Verify(scFrom) == nil {
		t.Log("[pormanager] verify successfully 2")
	} else {
		t.Error("[pormanager] verify failed 2")
	}

	if pmTo.IsFinal(scFrom) {
		t.Log("[pormanager] IsFinal test successfully 2")
	} else {
		t.Error("[pormanager] IsFinal test failed 2")
	}

	sig, _ := pmTo.GetSignature(scFrom)
	t.Log("[pormanager] GetSignature ", sig)

}
