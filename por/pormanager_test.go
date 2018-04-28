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
	if ret.Verify() == nil {
		fmt.Println("[pormanager] verify successfully")
	} else {
		fmt.Println("[pormanager] verify failed")
	}

}
