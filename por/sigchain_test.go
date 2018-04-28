package por

import (
	"fmt"
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
	sc, _ := NewSigChain(from, 1, &common.Uint256{1, 2, 3}, to.PubKey(), rel1.PubKey())
	if sc.Verify() == nil {
		fmt.Println("[sigchain] verify successfully")
	} else {
		fmt.Println("[sigchain] verify failed")
	}
	pm := NewPorManager(rel1)
	ret := pm.Sign(sc, rel2.PubKey())
	if ret.Verify() == nil {
		fmt.Println("[sigchain] verify successfully 2")
	} else {
		fmt.Println("[sigchain] verify failed 2")
	}

	pm2 := NewPorManager(rel2)
	ret2 := pm2.Sign(ret, to.PubKey())
	if ret2.Verify() == nil {
		fmt.Println("[sigchain] verify successfully 3")
	} else {
		fmt.Println("[sigchain] verify failed 3")
	}

	pm3 := NewPorManager(to)
	ret3 := pm3.Sign(ret, to.PubKey())
	if ret3.Verify() == nil {
		fmt.Println("[sigchain] verify successfully 4")
	} else {
		fmt.Println("[sigchain] verify failed 4")
	}

	//test Path()
	pks := ret3.Path()
	if crypto.Equal(pks[0], from.PubKey()) && crypto.Equal(pks[1], rel1.PubKey()) &&
		crypto.Equal(pks[2], rel2.PubKey()) && crypto.Equal(pks[3], to.PubKey()) {
		fmt.Println("[sigchain] Path test successfully")
	} else {
		fmt.Println("[sigchain] Path test failed")
	}

	dataHash := ret3.GetDataHash()
	if dataHash.CompareTo(common.Uint256{1, 2, 3}) == 0 {
		fmt.Println("[sigchain] GetDataHash test successfully")
	} else {
		fmt.Println("[sigchain] GetDataHash test failed")
	}

	if ret3.IsFinal() {
		fmt.Println("[sigchain] IsFinal test successfully")
	} else {
		fmt.Println("[sigchain] IsFinal test failed")
	}

	if ret3.Length() == 4 {
		fmt.Println("[sigchain] Length test successfully")
	} else {
		fmt.Println("[sigchain] Length test failed")
	}

	idx, err := ret3.GetSignerIndex(rel2.PubKey())
	if idx == 2 && err == nil {
		fmt.Println("[sigchain] GetSignerIndex test successfully")
	} else {
		fmt.Println("[sigchain] GetSignerIndex test failed")
	}

}
