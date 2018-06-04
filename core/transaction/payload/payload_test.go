package payload

import (
	"math/big"
	"testing"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/asset"
	"github.com/nknorg/nkn/crypto"
)

func TestBookKeeping(t *testing.T) {
	bk := &BookKeeping{
		Nonce: 10,
	}

	data, err := bk.MarshalJson()
	if err != nil {
		t.Error("BookKeeping MarshalJson error")
	}

	newBk := new(BookKeeping)
	err = newBk.UnmarshalJson(data)
	if err != nil {
		t.Error("BookKeeping UnmarshalJson error")
	}

	if !newBk.Equal(bk) {
		t.Error("BookKeeping not equal")
	}
}

func TestRegiserAsset(t *testing.T) {
	ra := &RegisterAsset{
		Asset: &asset.Asset{
			Name:        "hello",
			Description: "world",
			Precision:   8,
			AssetType:   0x11,
			RecordType:  0x1,
		},
		Amount: 100,
		Issuer: &crypto.PubKey{
			X: big.NewInt(55),
			Y: big.NewInt(66),
		},
		Controller: common.Uint160{
			1, 1, 3, 5,
		},
	}

	data, err := ra.MarshalJson()
	if err != nil {
		t.Error("RegisterAsset MarshalJson error")
	}

	newRa := new(RegisterAsset)
	err = newRa.UnmarshalJson(data)
	if err != nil {
		t.Error("RegisterAsset UnmarshalJson error")
	}

	if !newRa.Equal(ra) {
		t.Log(ra.Issuer, newRa.Issuer)
		t.Error("RegisterAsset not equal")
	}
}

func TestPrepaid(t *testing.T) {
	pp := &Prepaid{
		Asset:  common.Uint256{1, 2, 5, 7},
		Amount: 1000232,
		Rates:  345,
	}

	data, err := pp.MarshalJson()
	if err != nil {
		t.Error("Prepaid MarshalJson error")
	}

	newPp := new(Prepaid)
	err = newPp.UnmarshalJson(data)
	if err != nil {
		t.Error("Prepaid UnmarshalJson error")
	}

	if !newPp.Equal(pp) {
		t.Log(pp, newPp)
		t.Error("Prepaid not equal")
	}
}

func TestWithdraw(t *testing.T) {
	wd := &Withdraw{
		ProgramHash: common.Uint160{5, 4, 3, 2, 1},
	}

	data, err := wd.MarshalJson()
	if err != nil {
		t.Error("Withdraw MarshalJson error")
	}

	newWd := new(Withdraw)
	err = newWd.UnmarshalJson(data)
	if err != nil {
		t.Error("Withdraw UnmarshalJson error")
	}

	if !newWd.Equal(wd) {
		t.Log(wd, newWd)
		t.Error("Withdraw not equal")
	}
}

func TestCommit(t *testing.T) {
	cm := &Commit{
		SigChain:  []byte{7, 7, 49},
		Submitter: common.Uint160{5, 4, 3, 2, 1},
	}

	data, err := cm.MarshalJson()
	if err != nil {
		t.Error("Commit MarshalJson error")
	}

	newCm := new(Commit)
	err = newCm.UnmarshalJson(data)
	if err != nil {
		t.Error("Commit UnmarshalJson error")
	}

	if !newCm.Equal(cm) {
		t.Log(cm, newCm)
		t.Error("Commit not equal")
	}
}
