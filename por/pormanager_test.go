package por

import (
	"reflect"
	"testing"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/wallet"
)

func TestPorManager(t *testing.T) {
	crypto.SetAlg("P256R1")
	from, _ := wallet.NewAccount()
	rel, _ := wallet.NewAccount()
	to, _ := wallet.NewAccount()
	pmFrom := NewPorManager(from)
	pmRel := NewPorManager(rel)
	pmTo := NewPorManager(to)
	scFrom, err := pmFrom.CreateSigChain(1, &common.Uint256{}, to.PubKey(), rel.PubKey())
	if err != nil {
		t.Error("sigchain created failed")
	}
	scRel := pmRel.Sign(scFrom, to.PubKey())
	if pmRel.Verify(scRel) {
		t.Log("[pormanager] verify successfully")
	} else {
		t.Error("[pormanager] verify failed")
	}

	if !pmTo.IsFinal(scRel) {
		t.Log("[pormanager] IsFinal test successfully")
	} else {
		t.Error("[pormanager] IsFinal test failed")
	}

	scTo := pmTo.Sign(scRel, to.PubKey())
	if pmTo.Verify(scTo) {
		t.Log("[pormanager] verify successfully 2")
	} else {
		t.Error("[pormanager] verify failed 2")
	}

	if pmTo.IsFinal(scTo) {
		t.Log("[pormanager] IsFinal test successfully 2")
	} else {
		t.Error("[pormanager] IsFinal test failed 2")
	}

	elem, _ := scTo.lastSigElem()
	sig, err := pmTo.GetSignture(scTo)
	if reflect.DeepEqual(sig, elem.signature) != true || err != nil {
		t.Error("[pormanager] GetSignture error")
	} else {
		t.Log("[pormanager] GetSignture successfully")
	}

}
