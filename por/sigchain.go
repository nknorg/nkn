package por

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
)

// for the first relay node
// 1. NewSigChain : create a new Signature Chain and sign
//
// for the next relay node
// 1. Sign: sign the element created in Sign

// TODO: move sigAlgo to config.json
const sigAlgo SigAlgo = SigAlgo_ECDSA

func (sc *SigChainElem) SerializationUnsigned(w io.Writer) error {
	err := serialization.WriteVarBytes(w, sc.NextPubkey)
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

	err = serialization.WriteVarBytes(w, sc.DataHash)
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

func NewSigChainWithSignature(dataSize uint32, dataHash, blockHash, srcPubkey, destPubkey, nextPubkey, signature []byte, algo SigAlgo) (*SigChain, error) {
	sc := &SigChain{
		DataSize:   dataSize,
		DataHash:   dataHash,
		BlockHash:  blockHash,
		SrcPubkey:  srcPubkey,
		DestPubkey: destPubkey,
		Elems: []*SigChainElem{
			&SigChainElem{
				SigAlgo:    algo,
				NextPubkey: nextPubkey,
				Signature:  signature,
			},
		},
	}
	return sc, nil
}

// first relay node starts a new signature chain which consists of meta data and the first element.
func NewSigChain(owner *wallet.Account, dataSize uint32, dataHash, blockHash, destPubkey, nextPubkey []byte) (*SigChain, error) {
	ownPk := owner.PubKey()
	srcPubkey, err := ownPk.EncodePoint(true)
	if err != nil {
		return nil, err
	}

	sc := &SigChain{
		DataSize:   dataSize,
		DataHash:   dataHash,
		BlockHash:  blockHash,
		SrcPubkey:  srcPubkey,
		DestPubkey: destPubkey,
		Elems: []*SigChainElem{
			&SigChainElem{
				SigAlgo:    sigAlgo,
				NextPubkey: nextPubkey,
			},
		},
	}

	b := make([]byte, 4)
	_, err = rand.Read(b)
	if err != nil {
		return nil, err
	}
	sc.Nonce = binary.LittleEndian.Uint32(b)

	buff := bytes.NewBuffer(nil)
	if err := sc.SerializationMetadata(buff); err != nil {
		return nil, err
	}
	elem := sc.Elems[0]
	if err := elem.SerializationUnsigned(buff); err != nil {
		return nil, err
	}

	hash := sha256.Sum256(buff.Bytes())
	signature, err := crypto.Sign(owner.PrivKey(), hash[:])
	if err != nil {
		return nil, err
	}
	sc.Elems[0].Signature = signature

	return sc, nil
}

func NewSigChainElem(nextPubkey []byte) *SigChainElem {
	return &SigChainElem{
		SigAlgo:    sigAlgo,
		NextPubkey: nextPubkey,
		Signature:  nil,
	}
}

func (sc *SigChain) ExtendElement(nextPubkey []byte) ([]byte, error) {
	elem := NewSigChainElem(nextPubkey)
	lastElem, err := sc.lastSigElem()
	if err != nil {
		return nil, err
	}
	buff := bytes.NewBuffer(lastElem.Signature)
	err = elem.SerializationUnsigned(buff)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(buff.Bytes())
	sc.Elems = append(sc.Elems, elem)
	return hash[:], nil
}

func (sc *SigChain) AddLastSignature(signature []byte) error {
	lastElem, err := sc.lastSigElem()
	if err != nil {
		return err
	}
	if lastElem.Signature != nil {
		return errors.New("Last signature is already set")
	}
	lastElem.Signature = signature
	return nil
}

// Sign new created signature chain with local wallet.
func (sc *SigChain) Sign(nextPubkey []byte, signer *wallet.Account) error {
	sigNum := sc.Length()
	if sigNum < 1 {
		return errors.New("there are not enough signatures")
	}

	if err := sc.Verify(); err != nil {
		log.Error("Signature chain verification error:", err)
		return err
	}

	pk, err := sc.nextSigner()
	if err != nil {
		log.Error("Get next signer error:", err)
		return err
	}

	//TODO decode nextpk or encode signer pubkey
	nxPk, err := crypto.DecodePoint(pk)
	if err != nil {
		log.Error("Next publick key decoding error:", err)
		return errors.New("the next pubkey is wrong")
	}

	if !crypto.Equal(signer.PubKey(), nxPk) {
		return errors.New("signer is not the right one")
	}

	digest, err := sc.ExtendElement(nextPubkey)
	if err != nil {
		log.Error("Signature chain extent element error:", err)
		return err
	}

	signature, err := crypto.Sign(signer.PrivKey(), digest)
	if err != nil {
		log.Error("Compute signature error:", err)
		return err
	}

	err = sc.AddLastSignature(signature)
	if err != nil {
		log.Error("Add last signature error:", err)
		return err
	}

	return nil
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
			log.Error("Decode public key error:", err)
			return errors.New("the pubkey of e is wrong")
		}

		// verify each element signature
		// skip first and last element for now, will remove this once client
		// side signature is ready
		if i > 0 && !(sc.IsFinal() && i == sc.Length()-1) {
			buff := bytes.NewBuffer(prevSig)
			e.SerializationUnsigned(buff)
			currHash := sha256.Sum256(buff.Bytes())
			err = crypto.Verify(*ePk, currHash[:], e.Signature)
			if err != nil {
				log.Error("Verify signature error:", err)
				return err
			}
		}

		prevNextPubkey = e.NextPubkey
		prevSig = e.Signature
	}

	return nil
}

// Path returns signer path in signature chain.
func (sc *SigChain) Path() [][]byte {
	publicKeys := [][]byte{sc.SrcPubkey}
	for _, e := range sc.Elems {
		publicKeys = append(publicKeys, e.NextPubkey)
	}

	return publicKeys
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
	if len(sc.Elems) < 2 || !common.IsEqualBytes(sc.DestPubkey, sc.Elems[len(sc.Elems)-2].NextPubkey) {
		return false
	}
	return true
}

func (sc *SigChain) getElemByPubkey(pubkey []byte) (*SigChainElem, int, error) {
	if sc == nil || len(sc.Elems) == 0 {
		return nil, 0, errors.New("nil signature chain")
	}
	if common.IsEqualBytes(sc.SrcPubkey, pubkey) {
		return sc.Elems[0], 0, nil
	}

	for i, elem := range sc.Elems {
		if common.IsEqualBytes(elem.NextPubkey, pubkey) {
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

func (sc *SigChain) GetLastPubkey() ([]byte, error) {
	e, err := sc.lastSigElem()
	if err != nil {
		return nil, err
	}
	return e.NextPubkey, nil

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

func (sc *SigChain) SignatureHash() ([]byte, error) {
	signature, err := sc.GetSignature()
	if err != nil {
		return nil, err
	}
	sigHash := sha256.Sum256(signature)
	return sigHash[:], nil
}

func (sc *SigChain) GetBlockHeight() (*uint32, error) {
	blockHash, err := common.Uint256ParseFromBytes(sc.BlockHash)
	if err != nil {
		log.Error("Parse block hash uint256 from bytes error:", err)
		return nil, err
	}
	blockHeader, err := ledger.DefaultLedger.Store.GetHeader(blockHash)
	if err != nil {
		log.Error("Get block header error:", err)
		return nil, err
	}
	return &blockHeader.Height, nil
}

func (sc *SigChain) GetOwner() ([]byte, error) {
	if sc.IsFinal() {
		if pk, err := sc.GetLastPubkey(); err != nil {
			return []byte{}, err
		} else {
			return pk, nil
		}
	}

	return []byte{}, errors.New("no owner")

}

func (sc *SigChain) DumpInfo() {
	log.Info("dataSize: ", sc.DataSize)
	log.Info("dataHash: ", sc.DataHash)
	log.Info("srcPubkey: ", common.BytesToHexString(sc.SrcPubkey))
	log.Info("dstPubkey: ", common.BytesToHexString(sc.DestPubkey))

	for i, e := range sc.Elems {
		log.Info("nextPubkey[%d]: %s\n", i, common.BytesToHexString(e.NextPubkey))
		log.Info("signature[%d]: %s\n", i, common.BytesToHexString(e.Signature))
	}
}
