package por

import (
	"bytes"
	"testing"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/wallet"
)

func TestSigChain(t *testing.T) {
	crypto.SetAlg("P256R1")

	from, _ := wallet.NewAccount()
	to, _ := wallet.NewAccount()
	rel1, _ := wallet.NewAccount()
	rel2, _ := wallet.NewAccount()

	fromPk, _ := from.PubKey().EncodePoint(true)
	toPk, _ := to.PubKey().EncodePoint(true)
	rel1Pk, _ := rel1.PubKey().EncodePoint(true)
	rel2Pk, _ := rel2.PubKey().EncodePoint(true)

	//test Sign & Verify
	sc, err := NewSigChain(from, 1, 1, &common.Uint256{1, 2, 3}, toPk, rel1Pk)
	if err != nil || sc.Verify() != nil {
		t.Error("[TestSigChain] 'from' create new SigChain in error")
	}

	err = sc.Sign(rel2Pk, rel1)
	if err != nil || sc.Verify() != nil {
		t.Error("[TestSigChain] 'rel1' sign in error")
	}

	err = sc.Sign(toPk, rel2)
	if err != nil || sc.Verify() != nil {
		t.Error("[TestSigChain] 'rel2' sign in error")
	}

	err = sc.Sign(toPk, to)
	if err != nil || sc.Verify() != nil {
		t.Error("[TestSigChain] 'to' sign in error")
	}

	//test Path()
	pks := sc.Path()
	if !common.IsEqualBytes(pks[0], fromPk) ||
		!common.IsEqualBytes(pks[1], rel1Pk) ||
		!common.IsEqualBytes(pks[2], rel2Pk) ||
		!common.IsEqualBytes(pks[3], toPk) {
		t.Error("[TestSigChain] path of 'sc' is incorrect")
	}

	// test Length
	if sc.Length() != 4 {
		t.Error("[TestSigChain] length of 'sc' is incorrect")
	}

	// test IsFinal
	if !sc.IsFinal() {
		t.Error("[TestSigChain] IsFinal test failed")
	}

	// test GetSignerIndex
	idx, err := sc.GetSignerIndex(rel2Pk)
	if err != nil || idx != 2 {
		t.Error("[TestSigChain] GetSignerIndex test failed")
	}

	// test GetLastPubkey
	lpk, err := sc.GetLastPubkey()
	if !common.IsEqualBytes(lpk, toPk) {
		t.Error("[TestSigChain] GetLastPubkey test failed")
	}

	// test GetDataHash
	dataHash := sc.GetDataHash()
	if dataHash.CompareTo(common.Uint256{1, 2, 3}) != 0 {
		t.Error("[TestSigChain] GetDataHash test failed")
	}

	//test GetSignature
	if sig, err := sc.GetSignature(); err != nil || len(sig) != 64 {
		t.Error("[TestSigChain] Get GetSignature error", len(sig))
	}

	// test GetHeight
	if sc.GetHeight() != 1 {
		t.Error("[TestSigChain] GetHeight test failed")
	}

	// test GetOwner
	scOwner, err := sc.GetOwner()
	if err != nil {
		t.Error(err)
	}
	if !common.IsEqualBytes(scOwner, toPk) {
		t.Error("[TestSigChain] GetOwner test failed")
	}

	// test Serialize & Deserialize & Hash
	var sd SigChain
	buff := bytes.NewBuffer(nil)
	sc.Serialize(buff)
	sd.Deserialize(buff)
	if scHash := sc.Hash(); scHash.CompareTo(sd.Hash()) != 0 {
		t.Error("[TestSigChain] Serialize test failed")
	}

}
