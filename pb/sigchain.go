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
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nnet/overlay/chord"
)

// TODO: move sigAlgo to config.json
const (
	sigAlgo                    = VRF
	bitShiftPerSigChainElement = 4
)

func (sce *SigChainElem) SerializationUnsigned(w io.Writer) error {
	err := serialization.WriteVarBytes(w, sce.Addr)
	if err != nil {
		return err
	}

	err = serialization.WriteBool(w, sce.Mining)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, sce.NextPubkey)
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

	err = serialization.WriteVarBytes(w, sc.SrcPubkey)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, sc.BlockHash)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, sc.SrcPubkey)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, sc.DestPubkey)
	if err != nil {
		return err
	}

	return nil
}

func NewSigChainWithSignature(dataSize uint32, blockHash, srcID, srcPubkey, destPubkey, nextPubkey,
	signature []byte, algo SigAlgo, mining bool) (*SigChain, error) {
	sc := &SigChain{
		DataSize:   dataSize,
		BlockHash:  blockHash,
		SrcPubkey:  srcPubkey,
		DestPubkey: destPubkey,
		Elems: []*SigChainElem{
			{
				Addr:       srcID,
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

func NewSigChainElem(addr, nextPubkey, signature, vrf, proof []byte, mining bool) *SigChainElem {
	return &SigChainElem{
		Addr:       addr,
		SigAlgo:    sigAlgo,
		NextPubkey: nextPubkey,
		Signature:  signature,
		Vrf:        vrf,
		Proof:      proof,
		Mining:     mining,
	}
}

func ComputeSignature(secret, lastSignature, addr, nextPubkey []byte, mining bool) ([]byte, error) {
	elem := NewSigChainElem(addr, nextPubkey, nil, nil, nil, mining)
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

// Verify returns result of signature chain verification.
func (sc *SigChain) Verify() error {
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
		if i > 0 && !(sc.IsFinal() && i == sc.Length()-1) {
			expectedSignature, err := ComputeSignature(e.Vrf, prevSig, e.Addr, e.NextPubkey, e.Mining)
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
	if !sc.IsFinal() {
		return fmt.Errorf("signature chain is not final")
	}

	if len(sc.Elems) < 3 {
		return fmt.Errorf("signature chain should have at least 3 elements, but only has %d", len(sc.Elems))
	}

	var t big.Int
	lastNodeAddr := sc.Elems[len(sc.Elems)-2].Addr
	prevDistance := chord.Distance(sc.Elems[1].Addr, lastNodeAddr, config.NodeIDBytes*8)
	for i := 2; i < len(sc.Elems)-2; i++ {
		dist := chord.Distance(sc.Elems[i].Addr, lastNodeAddr, config.NodeIDBytes*8)
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

// firstSigElem returns the first element in signature chain.
func (sc *SigChain) firstSigElem() (*SigChainElem, error) {
	if sc == nil || len(sc.Elems) == 0 {
		return nil, errors.New("nil signature chain")
	}

	return sc.Elems[0], nil
}

// lastSigElem returns the last element in signature chain.
func (sc *SigChain) lastSigElem() (*SigChainElem, error) {
	if sc == nil || len(sc.Elems) == 0 {
		return nil, errors.New("nil signature chain")
	}

	return sc.Elems[sc.Length()-1], nil
}

func (sc *SigChain) finalSigElem() (*SigChainElem, error) {
	if !sc.IsFinal() {
		return nil, errors.New("not final")
	}

	n := sc.Length()
	if n < 2 {
		return nil, errors.New("not enough elements")
	}

	return sc.Elems[n-2], nil
}

func (sc *SigChain) IsFinal() bool {
	if sc.Length() < 3 {
		return false
	}
	pk := sc.Elems[len(sc.Elems)-2].NextPubkey
	return pk == nil || bytes.Equal(pk, sc.DestPubkey)
}

func (sc *SigChain) getElemByPubkey(pubkey []byte) (*SigChainElem, int, error) {
	if sc == nil || len(sc.Elems) == 0 {
		return nil, 0, errors.New("nil signature chain")
	}

	if bytes.Equal(sc.SrcPubkey, pubkey) {
		return sc.Elems[0], 0, nil
	}

	if sc.IsFinal() && bytes.Equal(pubkey, sc.DestPubkey) {
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
	if sc == nil || len(sc.Elems) < idx {
		return nil, errors.New("nil signature chain")
	}

	return sc.Elems[idx], nil
}

func (sc *SigChain) GetSignerIndex(pubkey []byte) (int, error) {
	_, idx, err := sc.getElemByPubkey(pubkey)
	return idx, err
}

func (sc *SigChain) GetMiner() ([]byte, []byte, error) {
	if !sc.IsFinal() {
		return nil, nil, errors.New("not final")
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
	elemLen := int64(len(minerElems))
	if elemLen == 0 {
		err := errors.New("invalid signature chain for block proposer selection")
		log.Error(err)
		return nil, nil, err
	}

	scSig, err := sc.GetSignature()
	if err != nil {
		return nil, nil, err
	}
	sigHashArray := sha256.Sum256(scSig)
	sigHash := sigHashArray[:]

	x := big.NewInt(0)
	x.SetBytes(sigHash)
	y := big.NewInt(elemLen)
	newIndex := big.NewInt(0)
	newIndex.Mod(x, y)

	originalIndex := minerElems[newIndex.Int64()].index
	if originalIndex == 0 {
		return sc.GetSrcPubkey(), sc.Elems[0].Addr, nil
	}

	return sc.Elems[originalIndex-1].NextPubkey, sc.Elems[originalIndex].Addr, nil
}

func (sc *SigChain) nextSigner() ([]byte, error) {
	e, err := sc.lastSigElem()
	if err != nil {
		return nil, errors.New("there is no elem")
	}
	return e.NextPubkey, nil
}

func (sc *SigChain) GetSignature() ([]byte, error) {
	sce, err := sc.finalSigElem()
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
