package sigchains

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
)

// for the first relay node
// 1. NewSigChain : create a new Signature Chain and sign
//
// for the next relay node
// 1. Sign: sign the element created in Sign

type SigChainEcdsa struct {
	nonce      [4]byte         // cryptographic nonce
	height     uint32          // blockchain height
	dataSize   uint32          // payload size
	dataHash   *common.Uint256 // payload hash
	srcPubkey  []byte          // source pubkey
	destPubkey []byte          // destination pubkey
	elems      []*SigChainElemEcdsa
}

type SigChainElemEcdsa struct {
	nextPubkey []byte // next signer
	signature  []byte // signature for signature chain element
}

// first relay node starts a new signature chain which consists of meta data and the first element.
func NewSigChainEcdsa(owner *wallet.Account, height, dataSize uint32, dataHash *common.Uint256, destPubkey, nextPubkey []byte) (*SigChainEcdsa, error) {
	ownPk := owner.PubKey()
	srcPubkey, err := ownPk.EncodePoint(true)
	if err != nil {
		return nil, err
	}

	sc := &SigChainEcdsa{
		height:     height,
		dataSize:   dataSize,
		dataHash:   dataHash,
		srcPubkey:  srcPubkey,
		destPubkey: destPubkey,
		elems: []*SigChainElemEcdsa{
			&SigChainElemEcdsa{
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

func NewSigChainElemEcdsa(nextPubkey []byte) *SigChainElemEcdsa {
	return &SigChainElemEcdsa{
		nextPubkey: nextPubkey,
		signature:  nil,
	}
}

// Sign new created signature chain with local wallet.
func (sce *SigChainEcdsa) Sign(nextPubkey []byte, signer *wallet.Account) error {
	sigNum := sce.Length()
	if sigNum < 1 {
		return errors.New("there are not enough signatures")
	}

	if err := sce.Verify(); err != nil {
		return err
	}

	pk, err := sce.nextSigner()
	if err != nil {
		return err
	}

	//TODO decode nextpk or encode signer pubkey
	nxPk, err := crypto.DecodePoint(pk)
	if err != nil {
		return errors.New("the next pubkey is wrong")
	}

	if !crypto.Equal(signer.PubKey(), nxPk) {
		return errors.New("signer is not the right one")
	}

	lastElem, err := sce.lastSigElem()
	buff := bytes.NewBuffer(lastElem.signature)
	elem := NewSigChainElemEcdsa(nextPubkey)
	err = elem.SerializationUnsigned(buff)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(buff.Bytes())
	signature, err := crypto.Sign(signer.PrivKey(), hash[:])
	if err != nil {
		return err
	}
	elem.signature = signature
	sce.elems = append(sce.elems, elem)

	return nil
}

// Verify returns result of signature chain verification.
func (sce *SigChainEcdsa) Verify() error {

	prevNextPubkey := sce.srcPubkey
	buff := bytes.NewBuffer(nil)
	sce.SerializationMetadata(buff)
	prevSig := buff.Bytes()
	for _, e := range sce.elems {
		ePk, err := crypto.DecodePoint(prevNextPubkey)
		if err != nil {
			return errors.New("the pubkey of e is wrong")
		}

		// verify each element signature
		buff := bytes.NewBuffer(prevSig)
		//	serialization.WriteVarBytes(buff, prevSig)
		e.SerializationUnsigned(buff)
		currHash := sha256.Sum256(buff.Bytes())
		err = crypto.Verify(*ePk, currHash[:], e.signature)
		if err != nil {
			return err
		}

		prevNextPubkey = e.nextPubkey
		prevSig = e.signature
	}

	return nil
}

// Path returns signer path in signature chain.
func (sce *SigChainEcdsa) Path() [][]byte {
	publicKeys := [][]byte{sce.srcPubkey}
	for _, e := range sce.elems {
		publicKeys = append(publicKeys, e.nextPubkey)
	}

	return publicKeys
}

// Length returns element num in current signature chain
func (sce *SigChainEcdsa) Length() int {
	return len(sce.elems)
}

func (sce *SigChainEcdsa) GetDataHash() *common.Uint256 {
	return sce.dataHash
}

// firstSigElem returns the first element in signature chain.
func (sce *SigChainEcdsa) firstSigElem() (*SigChainElemEcdsa, error) {
	if sce == nil || len(sce.elems) == 0 {
		return nil, errors.New("nil signature chain")
	}

	return sce.elems[0], nil
}

// lastSigElem returns the last element in signature chain.
func (sce *SigChainEcdsa) lastSigElem() (*SigChainElemEcdsa, error) {
	if sce == nil || len(sce.elems) == 0 {
		return nil, errors.New("nil signature chain")
	}
	num := len(sce.elems)

	return sce.elems[num-1], nil
}

func (sce *SigChainEcdsa) finalSigElem() (*SigChainElemEcdsa, error) {
	if len(sce.elems) < 2 && !common.IsEqualBytes(sce.destPubkey, sce.elems[len(sce.elems)-2].nextPubkey) {
		return nil, errors.New("unfinal")
	}

	return sce.elems[len(sce.elems)-1], nil
}

func (sce *SigChainEcdsa) IsFinal() bool {
	if len(sce.elems) < 2 || !common.IsEqualBytes(sce.destPubkey, sce.elems[len(sce.elems)-2].nextPubkey) {
		return false
	}
	return true
}

func (sce *SigChainEcdsa) getElemByPubkey(pubkey []byte) (*SigChainElemEcdsa, int, error) {
	if sce == nil || len(sce.elems) == 0 {
		return nil, 0, errors.New("nil signature chain")
	}
	if common.IsEqualBytes(sce.srcPubkey, pubkey) {
		return sce.elems[0], 0, nil
	}

	for i, elem := range sce.elems {
		if common.IsEqualBytes(elem.nextPubkey, pubkey) {
			return sce.elems[i+1], i + 1, nil
		}
	}

	return nil, 0, errors.New("not in sigchain")
}

func (sce *SigChainEcdsa) getElemByIndex(idx int) (*SigChainElemEcdsa, error) {
	if sce == nil || len(sce.elems) < idx {
		return nil, errors.New("nil signature chain")
	}

	return sce.elems[idx], nil
}

func (sce *SigChainEcdsa) GetSignerIndex(pubkey []byte) (int, error) {
	_, idx, err := sce.getElemByPubkey(pubkey)
	return idx, err
}
func (sce *SigChainEcdsa) GetLastPubkey() ([]byte, error) {
	e, err := sce.lastSigElem()
	if err != nil {
		return nil, err
	}
	return e.nextPubkey, nil

}

func (sce *SigChainEcdsa) nextSigner() ([]byte, error) {
	e, err := sce.lastSigElem()
	if err != nil {
		return nil, errors.New("there is no elem")
	}
	return e.nextPubkey, nil
}

func (sce *SigChainEcdsa) Serialize(w io.Writer) error {
	var err error
	err = sce.SerializationMetadata(w)
	if err != nil {
		return err
	}

	num := len(sce.elems)
	err = serialization.WriteUint32(w, uint32(num))
	if err != nil {
		return err
	}
	for i := 0; i < num; i++ {
		err = sce.elems[i].Serialize(w)
		if err != nil {
			return err
		}
	}

	return nil
}

func (sce *SigChainEcdsa) SerializationMetadata(w io.Writer) error {
	var err error

	err = serialization.WriteVarBytes(w, sce.nonce[:])
	if err != nil {
		return err
	}

	err = serialization.WriteUint32(w, sce.height)
	if err != nil {
		return err
	}

	err = serialization.WriteUint32(w, sce.dataSize)
	if err != nil {
		return err
	}

	_, err = sce.dataHash.Serialize(w)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, sce.srcPubkey)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, sce.destPubkey)
	if err != nil {
		return err
	}

	return nil
}

func (sce *SigChainEcdsa) Deserialize(r io.Reader) error {
	var err error

	err = sce.DeserializationMetadata(r)
	if err != nil {
		return err
	}

	num, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	for i := 0; i < int(num); i++ {
		scee := new(SigChainElemEcdsa)
		err = scee.Deserialize(r)
		if err != nil {
			return err
		}
		sce.elems = append(sce.elems, scee)
	}

	return nil
}

func (sce *SigChainEcdsa) DeserializationMetadata(r io.Reader) error {
	var err error

	nonce, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	copy(sce.nonce[:], nonce)

	sce.height, err = serialization.ReadUint32(r)
	if err != nil {
		return err
	}

	sce.dataSize, err = serialization.ReadUint32(r)
	if err != nil {
		return err
	}

	sce.dataHash = new(common.Uint256)
	err = sce.dataHash.Deserialize(r)
	if err != nil {
		return err
	}

	sce.srcPubkey, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	sce.destPubkey, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	return nil
}

func (sce *SigChainElemEcdsa) Serialize(w io.Writer) error {
	var err error
	err = sce.SerializationUnsigned(w)
	if err != nil {
		return err
	}
	err = serialization.WriteVarBytes(w, sce.signature[:])
	if err != nil {
		return err
	}

	return nil
}

func (sce *SigChainElemEcdsa) SerializationUnsigned(w io.Writer) error {
	var err error

	err = serialization.WriteVarBytes(w, sce.nextPubkey)
	if err != nil {
		return err
	}

	return nil
}

func (sce *SigChainElemEcdsa) Deserialize(r io.Reader) error {
	var err error
	err = sce.DeserializationUnsigned(r)
	if err != nil {
		return err
	}
	signature, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	sce.signature = signature

	return nil
}

func (sce *SigChainElemEcdsa) DeserializationUnsigned(r io.Reader) error {
	var err error

	nextPubkey, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	sce.nextPubkey = nextPubkey

	return nil
}

func (sce *SigChainEcdsa) GetSignature() ([]byte, error) {
	scee, err := sce.finalSigElem()
	if err != nil {
		return nil, err
	}

	return scee.signature, nil
}

func (sce *SigChainEcdsa) Hash() common.Uint256 {
	buff := bytes.NewBuffer(nil)
	sce.Serialize(buff)
	return sha256.Sum256(buff.Bytes())
}

func (sce *SigChainEcdsa) GetHeight() uint32 {
	return sce.height
}

func (sce *SigChainEcdsa) GetOwner() ([]byte, error) {
	if sce.IsFinal() {
		if pk, err := sce.GetLastPubkey(); err != nil {
			return []byte{}, err
		} else {
			return pk, nil
		}
	}

	return []byte{}, errors.New("no owner")

}

func (sce *SigChainEcdsa) DumpInfo() {
	log.Info("dataSize: ", sce.dataSize)
	log.Info("dataHash: ", sce.dataHash)
	log.Info("srcPubkey: ", common.BytesToHexString(sce.srcPubkey))
	log.Info("dstPubkey: ", common.BytesToHexString(sce.destPubkey))

	for i, e := range sce.elems {
		log.Info("nextPubkey[%d]: %s\n", i, common.BytesToHexString(e.nextPubkey))
		log.Info("signature[%d]: %s\n", i, common.BytesToHexString(e.signature))
	}
}
