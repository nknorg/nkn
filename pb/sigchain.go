package pb

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nnet/overlay/chord"
)

// TODO: move sigAlgo to config.json
const (
	sigAlgo                    = VRF
	bitShiftPerSigChainElement = 4
)

func (sce *SigChainElem) SerializationUnsigned(w io.Writer) error {
	err := serialization.WriteVarBytes(w, sce.Id)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, sce.NextPubkey)
	if err != nil {
		return err
	}

	err = serialization.WriteBool(w, sce.Mining)
	if err != nil {
		return err
	}

	return nil
}

func (sc *SigChain) SerializationMetadata(w io.Writer) error {
	err := serialization.WriteUint32(w, sc.Nonce)
	if err != nil {
		return err
	}

	err = serialization.WriteUint32(w, sc.DataSize)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, sc.BlockHash)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, sc.SrcId)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, sc.SrcPubkey)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, sc.DestId)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, sc.DestPubkey)
	if err != nil {
		return err
	}

	return nil
}

func NewSigChainWithSignature(dataSize uint32, blockHash, srcID, srcPubkey, destID, destPubkey, nextPubkey,
	signature []byte, algo SigAlgo, mining bool) (*SigChain, error) {
	sc := &SigChain{
		DataSize:   dataSize,
		BlockHash:  blockHash,
		SrcId:      srcID,
		SrcPubkey:  srcPubkey,
		DestId:     destID,
		DestPubkey: destPubkey,
		Elems: []*SigChainElem{
			{
				Id:         srcID,
				NextPubkey: nextPubkey,
				SigAlgo:    algo,
				Signature:  signature,
				Vrf:        nil,
				Proof:      nil,
				Mining:     mining,
			},
		},
	}
	return sc, nil
}

func NewSigChainElem(id, nextPubkey, signature, vrf, proof []byte, mining bool) *SigChainElem {
	return &SigChainElem{
		Id:         id,
		SigAlgo:    sigAlgo,
		NextPubkey: nextPubkey,
		Signature:  signature,
		Vrf:        vrf,
		Proof:      proof,
		Mining:     mining,
	}
}

func ComputeSignature(secret, lastSignature, id, nextPubkey []byte, mining bool) ([]byte, error) {
	elem := NewSigChainElem(id, nextPubkey, nil, nil, nil, mining)
	buff := bytes.NewBuffer(lastSignature)
	err := elem.SerializationUnsigned(buff)
	if err != nil {
		return nil, err
	}
	_, err = buff.Write(secret)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(buff.Bytes())
	return hash[:], nil
}

func (sc *SigChain) Verify() error {
	if err := sc.VerifyPath(); err != nil {
		return err
	}

	if err := sc.VerifySignatures(); err != nil {
		return err
	}

	return nil
}

// VerifySignatures returns whether all signatures in sigchain are valid
func (sc *SigChain) VerifySignatures() error {
	prevNextPubkey := sc.SrcPubkey
	buff := bytes.NewBuffer(nil)
	sc.SerializationMetadata(buff)
	prevSig := buff.Bytes()
	for i, e := range sc.Elems {
		ePk, err := crypto.DecodePoint(prevNextPubkey)
		if err != nil {
			return fmt.Errorf("invalid pubkey: %v", err)
		}

		// verify each element signature
		// skip first and last element for now, will remove this once client
		// side signature is ready
		if i > 0 && !(sc.IsComplete() && i == sc.Length()-1) {
			expectedSignature, err := ComputeSignature(e.Vrf, prevSig, e.Id, e.NextPubkey, e.Mining)
			if err != nil {
				return fmt.Errorf("compute signature error: %v", err)
			}

			if !bytes.Equal(e.Signature, expectedSignature) {
				return fmt.Errorf("signature %x is different from expected value %x", e.Signature, expectedSignature)
			}

			ok := crypto.VerifyVrf(*ePk, sc.BlockHash, e.Vrf, e.Proof)
			if !ok {
				return fmt.Errorf("invalid vrf or proof")
			}
		}

		prevNextPubkey = e.NextPubkey
		if e.NextPubkey == nil && i == sc.Length()-2 {
			prevNextPubkey = sc.DestPubkey
		}
		prevSig = e.Signature
	}

	return nil
}

func (sc *SigChain) VerifyPath() error {
	if sc.Length() < 3 {
		return fmt.Errorf("sigchain should have at least 3 elements, but only has %d", sc.Length())
	}

	if !bytes.Equal(sc.SrcId, sc.Elems[0].Id) {
		return fmt.Errorf("sigchain has wrong src id")
	}

	if !sc.IsComplete() {
		return fmt.Errorf("sigchain is not complete")
	}

	if bytes.Equal(sc.Elems[0].Id, sc.Elems[1].Id) {
		return fmt.Errorf("sender and relayer 1 has the same ID")
	}

	var t big.Int
	lastNodeID := sc.Elems[sc.Length()-2].Id
	prevDistance := chord.Distance(sc.Elems[1].Id, lastNodeID, config.NodeIDBytes*8)
	for i := 2; i < sc.Length()-2; i++ {
		dist := chord.Distance(sc.Elems[i].Id, lastNodeID, config.NodeIDBytes*8)
		if dist.Cmp(prevDistance) == 0 {
			return fmt.Errorf("relayer %d and %d has the same ID", i-1, i)
		}
		(&t).Mul(dist, big.NewInt(2))
		if t.Cmp(prevDistance) > 0 {
			return fmt.Errorf("signature chain path is invalid")
		}
		prevDistance = dist
	}

	return nil
}

// Length returns element num in current signature chain
func (sc *SigChain) Length() int {
	return len(sc.Elems)
}

func (sc *SigChain) lastRelayElem() (*SigChainElem, error) {
	if !sc.IsComplete() {
		return nil, errors.New("sigchain is not complete")
	}

	n := sc.Length()
	if n < 2 {
		return nil, errors.New("not enough elements")
	}

	return sc.Elems[n-2], nil
}

func (sc *SigChain) IsComplete() bool {
	if sc.Length() < 3 {
		return false
	}

	if !bytes.Equal(sc.DestId, sc.Elems[sc.Length()-1].Id) {
		return false
	}

	pk := sc.Elems[sc.Length()-2].NextPubkey
	return pk == nil || bytes.Equal(pk, sc.DestPubkey)
}

func (sc *SigChain) getElemByPubkey(pubkey []byte) (*SigChainElem, int, error) {
	if sc == nil || sc.Length() == 0 {
		return nil, 0, errors.New("nil signature chain")
	}

	if bytes.Equal(sc.SrcPubkey, pubkey) {
		return sc.Elems[0], 0, nil
	}

	if sc.IsComplete() && bytes.Equal(pubkey, sc.DestPubkey) {
		return sc.Elems[sc.Length()-1], sc.Length() - 1, nil
	}

	for i, elem := range sc.Elems {
		if i < sc.Length()-1 && bytes.Equal(elem.NextPubkey, pubkey) {
			return sc.Elems[i+1], i + 1, nil
		}
	}

	return nil, 0, errors.New("not in sigchain")
}

func (sc *SigChain) getElemByIndex(idx int) (*SigChainElem, error) {
	if sc == nil {
		return nil, errors.New("nil signature chain")
	}

	if sc.Length() <= idx {
		return nil, errors.New("no such element")
	}

	return sc.Elems[idx], nil
}

func (sc *SigChain) GetSignerIndex(pubkey []byte) (int, error) {
	_, idx, err := sc.getElemByPubkey(pubkey)
	return idx, err
}

func (sc *SigChain) GetMiner() ([]byte, []byte, error) {
	if !sc.IsComplete() {
		return nil, nil, errors.New("sigchain is not complete")
	}

	n := sc.Length()
	if n < 3 {
		return nil, nil, errors.New("not enough elements")
	}

	type SigChainElemInfo struct {
		index int
		elem  *SigChainElem
	}
	var minerElems []*SigChainElemInfo
	for i, e := range sc.Elems {
		if i > 0 && i < sc.Length()-1 && e.Mining == true {
			t := &SigChainElemInfo{
				index: i,
				elem:  e,
			}
			minerElems = append(minerElems, t)
		}
	}
	elemLen := len(minerElems)
	if elemLen == 0 {
		return nil, nil, errors.New("no mining element")
	}

	sigHash, err := sc.SignatureHash()
	if err != nil {
		return nil, nil, err
	}

	x := big.NewInt(0)
	x.SetBytes(sigHash)
	y := big.NewInt(int64(elemLen))
	newIndex := big.NewInt(0)
	newIndex.Mod(x, y)

	originalIndex := minerElems[newIndex.Int64()].index
	if originalIndex == 0 {
		return sc.SrcPubkey, sc.Elems[0].Id, nil
	}

	return sc.Elems[originalIndex-1].NextPubkey, sc.Elems[originalIndex].Id, nil
}

func (sc *SigChain) GetSignature() ([]byte, error) {
	sce, err := sc.lastRelayElem()
	if err != nil {
		return nil, err
	}

	return sce.Signature, nil
}

func ComputeSignatureHash(signature []byte, sigChainLen int) []byte {
	sigHashArray := sha256.Sum256(signature)
	sigHash := sigHashArray[:]
	rightShiftBytes(sigHash, bitShiftPerSigChainElement*sigChainLen)
	return sigHash
}

func (sc *SigChain) SignatureHash() ([]byte, error) {
	signature, err := sc.GetSignature()
	if err != nil {
		return nil, err
	}
	signatureHash := ComputeSignatureHash(signature, sc.Length())
	return signatureHash, nil
}
