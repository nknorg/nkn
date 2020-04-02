package pb

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nnet/overlay/chord"
)

const (
	bitShiftPerSigChainElement = 4
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

func (sc *SigChain) Verify(height uint32) error {
	if err := sc.VerifyMeta(height); err != nil {
		return err
	}

	if err := sc.VerifyPath(); err != nil {
		return err
	}

	if err := sc.VerifySignatures(); err != nil {
		return err
	}

	return nil
}

func (sc *SigChain) VerifyMeta(height uint32) error {
	if sc.Length() < 3 {
		return fmt.Errorf("sigchain should have at least 3 elements, but only has %d", sc.Length())
	}

	if len(sc.Elems[0].Id) > 0 && !bytes.Equal(sc.SrcId, sc.Elems[0].Id) {
		return fmt.Errorf("sigchain has wrong src id")
	}

	if !sc.IsComplete() {
		return fmt.Errorf("sigchain is not complete")
	}

	if bytes.Equal(sc.Elems[0].Id, sc.Elems[1].Id) {
		return fmt.Errorf("src and first relayer has the same ID")
	}

	if bytes.Equal(sc.Elems[sc.Length()-1].Id, sc.Elems[sc.Length()-2].Id) {
		return fmt.Errorf("dest and last relayer has the same ID")
	}

	if len(sc.Elems[sc.Length()-1].NextPubkey) > 0 {
		return fmt.Errorf("next pubkey in last sigchain elem should be empty")
	}

	if sc.Elems[0].Mining {
		return fmt.Errorf("first sigchain element should have mining set to false")
	}

	if sc.Elems[sc.Length()-1].Mining {
		return fmt.Errorf("last sigchain element should have mining set to false")
	}

	for i, e := range sc.Elems {
		if i == 0 || i == sc.Length()-1 {
			if len(e.Vrf) > 0 {
				return fmt.Errorf("sigchain elem %d vrf should be empty", i)
			}
			if len(e.Proof) > 0 {
				return fmt.Errorf("sigchain elem %d proof should be empty", i)
			}
		} else {
			if len(e.Vrf) == 0 {
				return fmt.Errorf("sigchain elem %d vrf should not be empty", i)
			}
			if len(e.Proof) == 0 {
				return fmt.Errorf("sigchain elem %d proof should not be empty", i)
			}
		}
		if !config.AllowSigChainHashSignature.GetValueAtHeight(height) {
			if e.SigAlgo != SIGNATURE {
				return fmt.Errorf("sigchain elem %d sig algo should be %v", i, SIGNATURE)
			}
		}
	}

	return nil
}

// VerifySignatures returns whether all signatures in sigchain are valid
func (sc *SigChain) VerifySignatures() error {
	prevNextPubkey := sc.SrcPubkey
	buff := bytes.NewBuffer(nil)
	err := sc.SerializationMetadata(buff)
	if err != nil {
		return err
	}
	metaHash := sha256.Sum256(buff.Bytes())
	prevHash := metaHash[:]
	for i, e := range sc.Elems {
		err := crypto.CheckPublicKey(prevNextPubkey)
		if err != nil {
			return fmt.Errorf("invalid pubkey %x: %v", prevNextPubkey, err)
		}

		if sc.IsComplete() && i == sc.Length()-1 {
			h := sha256.Sum256(prevHash)
			prevHash = h[:]
		}

		hash, err := e.Hash(prevHash)
		if err != nil {
			return err
		}

		switch e.SigAlgo {
		case SIGNATURE:
			err = crypto.Verify(prevNextPubkey, hash, e.Signature)
			if err != nil {
				return fmt.Errorf("signature %x is invalid: %v", e.Signature, err)
			}
		case HASH:
			if !bytes.Equal(e.Signature, hash) {
				return fmt.Errorf("signature %x is different from expected value %x", e.Signature, hash[:])
			}
		default:
			return fmt.Errorf("unknown SigAlgo %v", e.SigAlgo)
		}

		if len(e.Vrf) > 0 {
			ok := crypto.VerifyVrf(prevNextPubkey, sc.BlockHash, e.Vrf, e.Proof)
			if !ok {
				return fmt.Errorf("invalid vrf or proof")
			}
			prevHash = hash
		} else {
			prevHash = e.Signature
		}

		prevNextPubkey = e.NextPubkey
		if sc.IsComplete() && i == sc.Length()-2 && len(e.NextPubkey) == 0 {
			prevNextPubkey = sc.DestPubkey
		}
	}

	return nil
}

func (sc *SigChain) VerifyPath() error {
	var t big.Int
	lastNodeID := sc.Elems[sc.Length()-2].Id
	prevDistance := chord.Distance(sc.Elems[1].Id, lastNodeID, config.NodeIDBytes*8)
	for i := 2; i < sc.Length()-1; i++ {
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

func (sc *SigChain) SignatureHash() ([]byte, error) {
	lastRelayHash, err := sc.LastRelayHash()
	if err != nil {
		return nil, err
	}
	sigHash := ComputeSignatureHash(lastRelayHash, sc.Length())
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

func ComputeSignatureHash(lastRelayHash []byte, sigChainLen int) []byte {
	h := sha256.Sum256(lastRelayHash)
	sigHash := h[:]
	rightShiftBytes(sigHash, bitShiftPerSigChainElement*sigChainLen)
	return sigHash
}
