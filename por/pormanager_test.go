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

	if !pmTo.IsFinal(scRel) {
		t.Log("[pormanager] IsFinal test successfully")
	} else {
		t.Error("[pormanager] IsFinal test failed")
	}

	scTo, _ := pmTo.Sign(scRel, toPk)
	if pmTo.Verify(scTo) == nil {
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
