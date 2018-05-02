package por

import (
	"testing"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/wallet"
)

func TestSign(t *testing.T) {
	crypto.SetAlg("P256R1")
	from, _ := wallet.NewAccount()
	to, _ := wallet.NewAccount()
	rel1, _ := wallet.NewAccount()
	rel2, _ := wallet.NewAccount()

	//test Sign & Verify
	fromPk, _ := from.PubKey().EncodePoint(true)
	toPk, _ := to.PubKey().EncodePoint(true)
	rel1Pk, _ := rel1.PubKey().EncodePoint(true)
	rel2Pk, _ := rel2.PubKey().EncodePoint(true)
	sc, _ := NewSigChain(from, 1, &common.Uint256{1, 2, 3}, toPk, rel1Pk)
	if sc.Verify() == nil {
		t.Log("[sigchain] verify successfully")
	} else {
		t.Error("[sigchain] verify failed")
	}
	pm := NewPorManager(rel1)
	ret := pm.Sign(sc, rel2Pk)
	if ret.Verify() == nil {
		t.Log("[sigchain] verify successfully 2")
	} else {
		t.Error("[sigchain] verify failed 2")
	}

	pm2 := NewPorManager(rel2)
	ret2 := pm2.Sign(ret, toPk)
	if ret2.Verify() == nil {
		t.Log("[sigchain] verify successfully 3")
	} else {
		t.Error("[sigchain] verify failed 3")
	}

	pm3 := NewPorManager(to)
	ret3 := pm3.Sign(ret, toPk)
	if ret3.Verify() == nil {
		t.Log("[sigchain] verify successfully 4")
	} else {
		t.Error("[sigchain] verify failed 4")
	}

	//test Path()
	pks := ret3.Path()
	if common.IsEqualBytes(pks[0], fromPk) && common.IsEqualBytes(pks[1], rel1Pk) &&
		common.IsEqualBytes(pks[2], rel2Pk) && common.IsEqualBytes(pks[3], toPk) {
		t.Log("[sigchain] Path test successfully")
	} else {
		t.Error("[sigchain] Path test failed")
	}

	dataHash := ret3.GetDataHash()
	if dataHash.CompareTo(common.Uint256{1, 2, 3}) == 0 {
		t.Log("[sigchain] GetDataHash test successfully")
	} else {
		t.Error("[sigchain] GetDataHash test failed")
	}

	if ret3.IsFinal() {
		t.Log("[sigchain] IsFinal test successfully")
	} else {
		t.Error("[sigchain] IsFinal test failed")
	}

	if ret3.Length() == 4 {
		t.Log("[sigchain] Length test successfully")
	} else {
		t.Error("[sigchain] Length test failed")
	}

	idx, err := ret3.GetSignerIndex(rel2Pk)
	if idx == 2 && err == nil {
		t.Log("[sigchain] GetSignerIndex test successfully")
	} else {
		t.Error("[sigchain] GetSignerIndex test failed")
	}

}
