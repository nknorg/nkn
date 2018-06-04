package por

import (
	"bytes"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/wallet"
)

func TestSigChain(t *testing.T) {
	crypto.SetAlg("P256R1")

	from, _ := wallet.NewAccount()
	to, _ := wallet.NewAccount()
	relay1, _ := wallet.NewAccount()
	relay2, _ := wallet.NewAccount()

	fromPk, _ := from.PubKey().EncodePoint(true)
	toPk, _ := to.PubKey().EncodePoint(true)
	relay1Pk, _ := relay1.PubKey().EncodePoint(true)
	relay2Pk, _ := relay2.PubKey().EncodePoint(true)

	// test Sign & Verify
	dataHash := common.Uint256{1, 2, 3}
	blockHash := common.Uint256{4, 5, 6}
	sc, err := NewSigChain(from, 1, dataHash[:], blockHash[:], toPk, relay1Pk)
	if err != nil || sc.Verify() != nil {
		t.Error("[TestSigChain] 'from' create new SigChain in error")
	}

	err = sc.Sign(relay2Pk, relay1)
	if err != nil || sc.Verify() != nil {
		t.Error("[TestSigChain] 'relay1' sign in error")
	}

	err = sc.Sign(toPk, relay2)
	if err != nil || sc.Verify() != nil {
		t.Error("[TestSigChain] 'relay2' sign in error")
	}

	err = sc.Sign(toPk, to)
	if err != nil || sc.Verify() != nil {
		t.Error("[TestSigChain] 'to' sign in error")
	}

	// test Path
	pks := sc.Path()
	if !common.IsEqualBytes(pks[0], fromPk) ||
		!common.IsEqualBytes(pks[1], relay1Pk) ||
		!common.IsEqualBytes(pks[2], relay2Pk) ||
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
	idx, err := sc.GetSignerIndex(relay2Pk)
	if err != nil || idx != 2 {
		t.Error("[TestSigChain] GetSignerIndex test failed")
	}

	// test GetLastPubkey
	lpk, err := sc.GetLastPubkey()
	if !common.IsEqualBytes(lpk, toPk) {
		t.Error("[TestSigChain] GetLastPubkey test failed")
	}

	// test GetDataHash
	gotDataHash := sc.GetDataHash()
	if bytes.Compare(dataHash[:], gotDataHash[:]) != 0 {
		t.Error("[TestSigChain] GetDataHash test failed")
	}

	//test GetSignature
	if sig, err := sc.GetSignature(); err != nil || len(sig) != 64 {
		t.Error("[TestSigChain] Get GetSignature error", len(sig))
	}

	// test GetBlockHash
	gotBlockHash := sc.GetBlockHash()
	if bytes.Compare(blockHash[:], gotBlockHash[:]) != 0 {
		t.Error("[TestSigChain] GetBlockHash test failed")
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
	sd := &SigChain{}
	buf, err := proto.Marshal(sc)
	err = proto.Unmarshal(buf, sd)
	scHash, err := sc.SignatureHash()
	sdHash, err := sd.SignatureHash()
	if err == nil || bytes.Compare(scHash[:], sdHash[:]) != 0 {
		t.Error("[TestSigChain] Serialize test failed")
	}

	elem2, err := sc.getElemByIndex(2)
	if err != nil {
		t.Error(err)
	}
	if !common.IsEqualBytes(elem2.Signature, sc.Elems[2].Signature) {
		t.Error("[TestSigChain] getElemByIndex error")
	}

}
