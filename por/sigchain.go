package por

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
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

type SigAlgo uint8

const (
	ECDSA SigAlgo = 0
)

// TODO: move sigAlgo to config.json
const sigAlgo SigAlgo = ECDSA

type SigChain struct {
	nonce      [4]byte         // cryptographic nonce
	dataSize   uint32          // payload size
	dataHash   *common.Uint256 // payload hash
	blockHash  *common.Uint256 // latest block hash
	srcPubkey  []byte          // source pubkey
	destPubkey []byte          // destination pubkey
	elems      []*SigChainElem
}

type SigChainElem struct {
	algo       SigAlgo // signature algorithm
	nextPubkey []byte  // next signer
	signature  []byte  // signature for signature chain element
}

func NewSigChainWithSignature(dataSize uint32, dataHash, blockHash *common.Uint256, srcPubkey, destPubkey, nextPubkey, signature []byte, algo SigAlgo) (*SigChain, error) {
	sc := &SigChain{
		dataSize:   dataSize,
		dataHash:   dataHash,
		blockHash:  blockHash,
		srcPubkey:  srcPubkey,
		destPubkey: destPubkey,
		elems: []*SigChainElem{
			&SigChainElem{
				algo:       algo,
				nextPubkey: nextPubkey,
				signature:  signature,
			},
		},
	}
	return sc, nil
}

// first relay node starts a new signature chain which consists of meta data and the first element.
func NewSigChain(owner *wallet.Account, dataSize uint32, dataHash, blockHash *common.Uint256, destPubkey, nextPubkey []byte) (*SigChain, error) {
	ownPk := owner.PubKey()
	srcPubkey, err := ownPk.EncodePoint(true)
	if err != nil {
		return nil, err
	}

	sc := &SigChain{
		dataSize:   dataSize,
		dataHash:   dataHash,
		blockHash:  blockHash,
		srcPubkey:  srcPubkey,
		destPubkey: destPubkey,
		elems: []*SigChainElem{
			&SigChainElem{
				algo:       sigAlgo,
				nextPubkey: nextPubkey,
			},
		},
	}

	rand.Read(sc.nonce[:])

	buff := bytes.NewBuffer(nil)
	if err := sc.SerializationMetadata(buff); err != nil {
		return nil, err
	}
	elem := sc.elems[0]
	if err := elem.SerializationUnsigned(buff); err != nil {
		return nil, err
	}

	hash := sha256.Sum256(buff.Bytes())
	signature, err := crypto.Sign(owner.PrivKey(), hash[:])
	if err != nil {
		return nil, err
	}
	sc.elems[0].signature = signature

	return sc, nil
}

func NewSigChainElem(nextPubkey []byte) *SigChainElem {
	return &SigChainElem{
		algo:       sigAlgo,
		nextPubkey: nextPubkey,
		signature:  nil,
	}
}

func (sc *SigChain) ExtendElement(nextPubkey []byte) ([]byte, error) {
	elem := NewSigChainElem(nextPubkey)
	lastElem, err := sc.lastSigElem()
	if err != nil {
		return nil, err
	}
	buff := bytes.NewBuffer(lastElem.signature)
	err = elem.SerializationUnsigned(buff)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(buff.Bytes())
	sc.elems = append(sc.elems, elem)
	return hash[:], nil
}

func (sc *SigChain) AddLastSignature(signature []byte) error {
	lastElem, err := sc.lastSigElem()
	if err != nil {
		return err
	}
	if lastElem.signature != nil {
		return errors.New("Last signature is already set")
	}
	lastElem.signature = signature
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
	prevNextPubkey := sc.srcPubkey
	buff := bytes.NewBuffer(nil)
	sc.SerializationMetadata(buff)
	prevSig := buff.Bytes()
	for i, e := range sc.elems {
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
			err = crypto.Verify(*ePk, currHash[:], e.signature)
			if err != nil {
				log.Error("Verify signature error:", err)
				return err
			}
		}

		prevNextPubkey = e.nextPubkey
		prevSig = e.signature
	}

	return nil
}

// Path returns signer path in signature chain.
func (sc *SigChain) Path() [][]byte {
	publicKeys := [][]byte{sc.srcPubkey}
	for _, e := range sc.elems {
		publicKeys = append(publicKeys, e.nextPubkey)
	}

	return publicKeys
}

// Length returns element num in current signature chain
func (sc *SigChain) Length() int {
	return len(sc.elems)
}

func (sc *SigChain) GetDataHash() *common.Uint256 {
	return sc.dataHash
}

func (sc *SigChain) GetSrcPubkey() []byte {
	return sc.srcPubkey
}

func (sc *SigChain) GetDestPubkey() []byte {
	return sc.destPubkey
}

// firstSigElem returns the first element in signature chain.
func (sc *SigChain) firstSigElem() (*SigChainElem, error) {
	if sc == nil || len(sc.elems) == 0 {
		return nil, errors.New("nil signature chain")
	}

	return sc.elems[0], nil
}

// secondLastSigElem returns the second last element in signature chain.
func (sc *SigChain) secondLastSigElem() (*SigChainElem, error) {
	if sc == nil || len(sc.elems) == 0 {
		return nil, errors.New("nil signature chain")
	}

	num := len(sc.elems)
	if num < 2 {
		return nil, errors.New("signature chain length less than 2")
	}

	return sc.elems[num-2], nil
}

// lastSigElem returns the last element in signature chain.
func (sc *SigChain) lastSigElem() (*SigChainElem, error) {
	if sc == nil || len(sc.elems) == 0 {
		return nil, errors.New("nil signature chain")
	}
	num := len(sc.elems)

	return sc.elems[num-1], nil
}

func (sc *SigChain) finalSigElem() (*SigChainElem, error) {
	if !sc.IsFinal() {
		return nil, errors.New("not final")
	}

	return sc.elems[len(sc.elems)-1], nil
}

func (sc *SigChain) IsFinal() bool {
	if len(sc.elems) < 2 || !common.IsEqualBytes(sc.destPubkey, sc.elems[len(sc.elems)-2].nextPubkey) {
		return false
	}
	return true
}

func (sc *SigChain) getElemByPubkey(pubkey []byte) (*SigChainElem, int, error) {
	if sc == nil || len(sc.elems) == 0 {
		return nil, 0, errors.New("nil signature chain")
	}
	if common.IsEqualBytes(sc.srcPubkey, pubkey) {
		return sc.elems[0], 0, nil
	}

	for i, elem := range sc.elems {
		if common.IsEqualBytes(elem.nextPubkey, pubkey) {
			return sc.elems[i+1], i + 1, nil
		}
	}

	return nil, 0, errors.New("not in sigchain")
}

func (sc *SigChain) getElemByIndex(idx int) (*SigChainElem, error) {
	if sc == nil || len(sc.elems) < idx {
		return nil, errors.New("nil signature chain")
	}

	return sc.elems[idx], nil
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
	return e.nextPubkey, nil

}

func (sc *SigChain) nextSigner() ([]byte, error) {
	e, err := sc.lastSigElem()
	if err != nil {
		return nil, errors.New("there is no elem")
	}
	return e.nextPubkey, nil
}

func (sc *SigChain) Serialize(w io.Writer) error {
	var err error
	err = sc.SerializationMetadata(w)
	if err != nil {
		return err
	}

	num := len(sc.elems)
	err = serialization.WriteUint32(w, uint32(num))
	if err != nil {
		return err
	}
	for i := 0; i < num; i++ {
		err = sc.elems[i].Serialize(w)
		if err != nil {
			return err
		}
	}

	return nil
}

func (sc *SigChain) SerializationMetadata(w io.Writer) error {
	var err error

	err = serialization.WriteVarBytes(w, sc.nonce[:])
	if err != nil {
		return err
	}

	err = serialization.WriteUint32(w, sc.dataSize)
	if err != nil {
		return err
	}

	_, err = sc.dataHash.Serialize(w)
	if err != nil {
		return err
	}

	_, err = sc.blockHash.Serialize(w)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, sc.srcPubkey)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, sc.destPubkey)
	if err != nil {
		return err
	}

	return nil
}

func (sc *SigChain) Deserialize(r io.Reader) error {
	var err error

	err = sc.DeserializationMetadata(r)
	if err != nil {
		return err
	}

	num, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	for i := 0; i < int(num); i++ {
		sce := new(SigChainElem)
		err = sce.Deserialize(r)
		if err != nil {
			return err
		}
		sc.elems = append(sc.elems, sce)
	}

	return nil
}

func (sc *SigChain) DeserializationMetadata(r io.Reader) error {
	var err error

	nonce, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	copy(sc.nonce[:], nonce)

	sc.dataSize, err = serialization.ReadUint32(r)
	if err != nil {
		return err
	}

	sc.dataHash = new(common.Uint256)
	err = sc.dataHash.Deserialize(r)
	if err != nil {
		return err
	}

	sc.blockHash = new(common.Uint256)
	err = sc.blockHash.Deserialize(r)
	if err != nil {
		return err
	}

	sc.srcPubkey, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	sc.destPubkey, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	return nil
}

func (sc *SigChainElem) Serialize(w io.Writer) error {
	var err error
	err = sc.SerializationUnsigned(w)
	if err != nil {
		return err
	}
	err = serialization.WriteVarBytes(w, sc.signature[:])
	if err != nil {
		return err
	}

	return nil
}

func (sc *SigChainElem) SerializationUnsigned(w io.Writer) error {
	var err error

	err = serialization.WriteVarBytes(w, sc.nextPubkey)
	if err != nil {
		return err
	}

	return nil
}

func (sc *SigChainElem) Deserialize(r io.Reader) error {
	var err error
	err = sc.DeserializationUnsigned(r)
	if err != nil {
		return err
	}
	signature, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	sc.signature = signature

	return nil
}

func (sc *SigChainElem) DeserializationUnsigned(r io.Reader) error {
	var err error

	nextPubkey, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	sc.nextPubkey = nextPubkey

	return nil
}

func (sc *SigChain) GetSignature() ([]byte, error) {
	sce, err := sc.finalSigElem()
	if err != nil {
		return nil, err
	}

	return sce.signature, nil
}

func (sc *SigChain) Hash() common.Uint256 {
	buff := bytes.NewBuffer(nil)
	sc.Serialize(buff)
	return sha256.Sum256(buff.Bytes())
}

func (sc *SigChain) GetBlockHash() *common.Uint256 {
	return sc.blockHash
}

func (sc *SigChain) GetBlockHeight() (*uint32, error) {
	blockHeader, err := ledger.DefaultLedger.Store.GetHeader(*sc.blockHash)
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
	log.Info("dataSize: ", sc.dataSize)
	log.Info("dataHash: ", sc.dataHash)
	log.Info("srcPubkey: ", common.BytesToHexString(sc.srcPubkey))
	log.Info("dstPubkey: ", common.BytesToHexString(sc.destPubkey))

	for i, e := range sc.elems {
		log.Info("nextPubkey[%d]: %s\n", i, common.BytesToHexString(e.nextPubkey))
		log.Info("signature[%d]: %s\n", i, common.BytesToHexString(e.signature))
	}
}
