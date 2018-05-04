package sigchains

import (
	"bytes"
	"testing"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/wallet"
)

func TestSigChainEcdsa(t *testing.T) {
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
	sc, _ := NewSigChainEcdsa(from, 1, &common.Uint256{1, 2, 3}, toPk, rel1Pk)
	if sc.Verify() == nil {
		t.Log("[sigchain] verify successfully")
	} else {
		t.Error("[sigchain] verify failed")
	}
	sc.Sign(rel2Pk, rel1)
	if sc.Verify() == nil {
		t.Log("[sigchain] verify successfully 2")
	} else {
		t.Error("[sigchain] verify failed 2")
	}

	sc.Sign(toPk, rel2)
	if sc.Verify() == nil {
		t.Log("[sigchain] verify successfully 3")
	} else {
		t.Error("[sigchain] verify failed 3")
	}

	sc.Sign(toPk, to)
	if sc.Verify() == nil {
		t.Log("[sigchain] verify successfully 4")
	} else {
		t.Error("[sigchain] verify failed 4")
	}

	//test Path()
	pks := sc.Path()
	if common.IsEqualBytes(pks[0], fromPk) && common.IsEqualBytes(pks[1], rel1Pk) &&
		common.IsEqualBytes(pks[2], rel2Pk) && common.IsEqualBytes(pks[3], toPk) {
		t.Log("[sigchain] Path test successfully")
	} else {
		t.Error("[sigchain] Path test failed")
	}

	dataHash := sc.GetDataHash()
	if dataHash.CompareTo(common.Uint256{1, 2, 3}) == 0 {
		t.Log("[sigchain] GetDataHash test successfully")
	} else {
		t.Error("[sigchain] GetDataHash test failed")
	}

	if sc.IsFinal() {
		t.Log("[sigchain] IsFinal test successfully")
	} else {
		t.Error("[sigchain] IsFinal test failed")
	}

	if sc.Length() == 4 {
		t.Log("[sigchain] Length test successfully")
	} else {
		t.Error("[sigchain] Length test failed")
	}

	idx, err := sc.GetSignerIndex(rel2Pk)
	if idx == 2 && err == nil {
		t.Log("[sigchain] GetSignerIndex test successfully")
	} else {
		t.Error("[sigchain] GetSignerIndex test failed")
	}

	buff := bytes.NewBuffer(nil)
	sc.Serialize(buff)
	var sd SigChainEcdsa
	sd.Deserialize(buff)

	scOwner, err := sc.GetOwner()
	if err != nil {
		t.Error(err)
	}
	sdOwner, err := sd.GetOwner()
	if err != nil {
		t.Error(err)
	}
	if !common.IsEqualBytes(scOwner, sdOwner) {
		t.Error("Serialize error")
	}

}
