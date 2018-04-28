package por

import (
	"fmt"
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
	pm := NewPorManager(rel)
	sc, _ := NewSigChain(from, 1, &common.Uint256{}, to.PubKey(), rel.PubKey())
	ret := pm.Sign(sc, to.PubKey())
	if pm.Verify(ret) {
		fmt.Println("[pormanager] verify successfully")
	} else {
		fmt.Println("[pormanager] verify failed")
	}
	pm2 := NewPorManager(to)
	ret2 := pm2.Sign(ret, to.PubKey())
	if pm2.Verify(ret2) {
		fmt.Println("[pormanager] verify successfully 2")
	} else {
		fmt.Println("[pormanager] verify failed 2")
	}

}
