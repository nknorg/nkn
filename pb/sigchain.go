package pb

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"

	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/common/serialization"
	"github.com/nknorg/nkn/v2/config"
)

func NewSigChainElem(id, nextPubkey, signature, vrf, proof []byte, mining bool, sigAlgo SigAlgo) *SigChainElem {
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

	if len(sce.Vrf) > 0 {
		_, err = w.Write(sce.Vrf)
		if err != nil {
			return err
		}
	}

	return nil
}

func (sce *SigChainElem) Hash(prevHash []byte) ([]byte, error) {
	buff := bytes.NewBuffer(prevHash)
	err := sce.SerializationUnsigned(buff)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(buff.Bytes())
	return hash[:], nil
}

func (sce *SigChainElem) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"id":         common.HexStr(sce.Id),
		"nextPubkey": common.HexStr(sce.NextPubkey),
		"mining":     sce.Mining,
		"signature":  common.HexStr(sce.Signature),
		"sigAlgo":    sce.SigAlgo,
		"vrf":        common.HexStr(sce.Vrf),
		"proof":      common.HexStr(sce.Proof),
	}
}

func NewSigChain(nonce, dataSize uint32, blockHash, srcID, srcPubkey, destID, destPubkey, nextPubkey, signature []byte, algo SigAlgo, mining bool) (*SigChain, error) {
	sc := &SigChain{
		Nonce:      nonce,
		DataSize:   dataSize,
		BlockHash:  blockHash,
		SrcId:      srcID,
		SrcPubkey:  srcPubkey,
		DestId:     destID,
		DestPubkey: destPubkey,
		Elems: []*SigChainElem{
			{
				Id:         nil,
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

// Length returns element num in current signature chain
func (sc *SigChain) Length() int {
	return len(sc.Elems)
}

func (sc *SigChain) lastRelayElem() (*SigChainElem, error) {
	if !sc.IsComplete() {
		return nil, errors.New("sigchain is not complete")
	}
	return sc.Elems[sc.Length()-2], nil
}

func (sc *SigChain) IsComplete() bool {
	if sc.Length() < 3 {
		return false
	}

	id := sc.Elems[sc.Length()-1].Id
	if len(id) > 0 && !bytes.Equal(id, sc.DestId) {
		return false
	}

	pk := sc.Elems[sc.Length()-2].NextPubkey
	return len(pk) == 0 || bytes.Equal(pk, sc.DestPubkey)
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

func (sc *SigChain) GetMiner(height uint32, randomBeacon []byte) ([]byte, []byte, error) {
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
			base := config.SigChainMinerWeightBase.GetValueAtHeight(height)
			exp := int(config.SigChainMinerWeightMaxExponent.GetValueAtHeight(height))
			if exp > i-1 {
				exp = i - 1
			}
			if exp > sc.Length()-2-i {
				exp = sc.Length() - 2 - i
			}
			count := int(math.Pow(float64(base), float64(exp)))
			for i := 0; i < count; i++ {
				minerElems = append(minerElems, t)
			}
		}
	}
	elemLen := len(minerElems)
	if elemLen == 0 {
		return nil, nil, errors.New("no mining element")
	}

	sigHash, err := sc.SignatureHash(height, 0)
	if err != nil {
		return nil, nil, err
	}

	h := sigHash
	if config.SigChainMinerSalt.GetValueAtHeight(height) {
		hs := sha256.Sum256(append(sigHash, randomBeacon...))
		h = hs[:]
	}

	x := big.NewInt(0)
	x.SetBytes(h)
	y := big.NewInt(int64(elemLen))
	newIndex := big.NewInt(0)
	newIndex.Mod(x, y)

	originalIndex := minerElems[newIndex.Int64()].index
	if originalIndex == 0 {
		return nil, nil, errors.New("invalid miner index")
	}

	return sc.Elems[originalIndex-1].NextPubkey, sc.Elems[originalIndex].Id, nil
}

func (sc *SigChain) LastRelayHash() ([]byte, error) {
	if !sc.IsComplete() {
		return nil, fmt.Errorf("sigchain is not complete")
	}

	buff := bytes.NewBuffer(nil)
	err := sc.SerializationMetadata(buff)
	if err != nil {
		return nil, err
	}
	metaHash := sha256.Sum256(buff.Bytes())
	prevHash := metaHash[:]

	for i := 0; i < sc.Length()-1; i++ {
		hash, err := sc.Elems[i].Hash(prevHash)
		if err != nil {
			return nil, err
		}

		if len(sc.Elems[i].Vrf) > 0 {
			prevHash = hash
		} else {
			prevHash = sc.Elems[i].Signature
		}
	}

	return prevHash, nil
}

func (sc *SigChain) SignatureHash(height uint32, leftShiftBit int) ([]byte, error) {
	lastRelayHash, err := sc.LastRelayHash()
	if err != nil {
		return nil, err
	}
	sigHash := ComputeSignatureHash(lastRelayHash, sc.Length(), height, leftShiftBit)
	return sigHash, nil
}

func (sc *SigChain) ToMap() map[string]interface{} {
	elems := make([]interface{}, 0)
	for _, e := range sc.Elems {
		elems = append(elems, e.ToMap())
	}
	return map[string]interface{}{
		"nonce":      sc.Nonce,
		"dataSize":   sc.DataSize,
		"blockHash":  common.HexStr(sc.BlockHash),
		"srcId":      common.HexStr(sc.SrcId),
		"srcPubkey":  common.HexStr(sc.SrcPubkey),
		"destId":     common.HexStr(sc.DestId),
		"destPubkey": common.HexStr(sc.DestPubkey),
		"elems":      elems,
	}
}

func ComputeSignatureHash(lastRelayHash []byte, sigChainLen int, height uint32, leftShiftBit int) []byte {
	h := sha256.Sum256(lastRelayHash)
	sigHash := h[:]
	maxSigChainLen := int(config.SigChainBitShiftMaxLength.GetValueAtHeight(height))
	if maxSigChainLen > 0 && sigChainLen > maxSigChainLen {
		sigChainLen = maxSigChainLen
	}
	rightShiftBytes(sigHash, int(config.SigChainBitShiftPerElement.GetValueAtHeight(height))*sigChainLen-leftShiftBit)
	return sigHash
}

func rightShiftBytes(b []byte, n int) {
	if n > 0 {
		for k := 0; k < n; k++ {
			b[len(b)-1] >>= 1
			for i := len(b) - 2; i >= 0; i-- {
				if b[i]&0x1 == 0x1 {
					b[i+1] |= 0x80
				}
				b[i] >>= 1
			}
		}
	} else if n < 0 {
		for k := 0; k < -n; k++ {
			if b[0]&0x80 == 0x80 {
				break
			}
			b[0] <<= 1
			for i := 1; i < len(b); i++ {
				if b[i]&0x80 == 0x80 {
					b[i-1] |= 0x1
				}
				b[i] <<= 1
			}
		}
	}
}
