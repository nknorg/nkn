package por

import (
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
	pm := NewPorManager(from)
	sc := New(1, &common.Uint256{}, from.PubKey(), to.PubKey(), rel.PubKey())
	sc.dump()
	ret := pm.Sign(sc, rel.PubKey())
	ret.dump()
}
