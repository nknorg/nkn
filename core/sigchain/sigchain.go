package sigchain

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"

	. "nkn-core/common"
	"nkn-core/common/serialization"
	"nkn-core/crypto"
)

type SigChain struct {
	dataSize   uint32         // payload size
	dataHash   *Uint256       // payload hash
	srcPubkey  *crypto.PubKey // source pubkey
	destPubkey *crypto.PubKey // destination pubkey
	elems      []*SigChainElem
}

type SigChainElem struct {
	pubkey     *crypto.PubKey // current signer
	nextPubkey *crypto.PubKey // next signer
	signature  []byte         // signature for signature chain element
}

// first relay node starts a new signature chain which consists of meta data and the first element.
func New(dataSize uint32, dataHash *Uint256, srcPubkey *crypto.PubKey, destPubkey *crypto.PubKey, nextPubkey *crypto.PubKey, signature []byte) *SigChain {
	var chain SigChain
	firstElem := &SigChainElem{
		pubkey:     pubKey,
		nextPubkey: nextPubkey,
		signature:  signature,
	}
	chain.dataSize = dataSize
	chain.dataHash = dataHash
	chain.srcPubkey = srcPubkey
	chain.destPubkey = destPubkey
	chain.elems = append(chain.elems, firstElem)

	return &chain
}

// Create a new signature chain element for relay node.
func NewElem(pubkey *crypto.PubKey, nextPubkey *crypto.PubKey, signature []byte) *SigChainElem {
	return &SigChainElem{
		pubkey:     pubKey,
		nextPubkey: nextPubkey,
		signature:  signature,
	}
}

// stub function, when receive data from other node, calculate hash first
// to construct signature chain element.
func CalculateDataHash(data []byte) (*Uint256, error) {
	h := sha256.Sum256(data)
	hash, err := Uint256ParseFromBytes(h[:])
	if err != nil {
		return nil, err
	}

	return &hash, nil
}

// stub function, sign signature chain element with local wallet.
func SignSigChainELem(elem *SigChainElem) ([]byte, error) {
	var err error
	var privateKeyStub = []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	}
	buff := bytes.NewBuffer(nil)
	err = elem.SerializationUnsigned(buff)
	if err != nil {
		return nil, err
	}

	return crypto.Sign(privateKeyStub, buff.Bytes())
}

// Append appends a new signature chain element in existed signature chain.
func (p *SigChain) Append(elem *SigChainElem) (*SigChain, error) {
	if p == nil {
		return nil, errors.New("append elem to nil chain")
	}
	if elem == nil {
		return p, nil
	}
	// verify full chain before appending
	if err := p.Verify(); err != nil {
		return nil, err
	}
	// verify new element with last script hash in signature chain
	lastElem, err := p.LastSigElem()
	if err != nil {
		return nil, err
	}
	if crypto.Equal(elem.pubkey, lastElem.nextPubkey) != true {
		return nil, errors.New("appending new element to signature chain error")
	}
	p.elems = append(p.elems, elem)

	return p, nil
}

// Verify returns result of signature chain verification.
func (p *SigChain) Verify() error {
	var err error

	firstElem, err := p.FirstSigElem()
	if err != nil {
		return err
	}

	prevNextPubkey := p.srcPubkey

	buff := bytes.NewBuffer(nil)
	p.SerializationMetadata(buff)
	prevHash := sha256.Sum256(buff.Bytes())

	for _, e := range p.elems {
		// verify each element public key is correct
		if crypto.Equal(prevNextPubkey, e.pubkey) != true {
			return errors.New("unmatch public key in signature chain")
		}

		// verify each element signature
		buff := bytes.NewBuffer(nil)
		e.SerializationUnsigned(buff)
		prevHash.Serialize(buff)
		currHash := sha256.Sum256(buff.Bytes())
		err = crypto.Verify(*e.pubkey, currHash[:], e.signature)
		if err != nil {
			return err
		}

		prevNextPubkey = e.nextPubkey
		prevHash = sha256.Sum256(e.signature)
	}

	return nil
}

// Path returns signer path in signature chain.
func (p *SigChain) Path() []*crypto.PubKey {
	var publicKeys []*crypto.PubKey
	for _, e := range p.elems {
		publicKeys = append(publicKeys, e.pubkey)
	}

	return publicKeys
}

// FirstSigElem returns the first element in signature chain.
func (p *SigChain) FirstSigElem() (*SigChainElem, error) {
	if p == nil || len(p.elems) == 0 {
		return nil, errors.New("nil signature chain")
	}

	return p.elems[0], nil
}

// LastSigElem returns the last element in signature chain.
func (p *SigChain) LastSigElem() (*SigChainElem, error) {
	if p == nil || len(p.elems) == 0 {
		return nil, errors.New("nil signature chain")
	}
	num := len(p.elems)

	return p.elems[num-1], nil
}

func (p *SigChain) Serialization(w io.Writer) error {
	var err error
	err = p.SerializationMetadata(w)
	if err != nil {
		return err
	}

	num := len(p.elems)
	err = serialization.WriteUint32(w, uint32(num))
	if err != nil {
		return err
	}
	for i := 0; i < num; i++ {
		err = p.elems[i].Serialization(w)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *SigChain) SerializationMetadata(w io.Writer) error {
	var err error
	err = serialization.WriteUint32(w, uint32(p.dataSize))
	if err != nil {
		return err
	}

	_, err = p.dataHash.Serialize(w)
	if err != nil {
		return err
	}

	err = p.srcPubkey.Serialize(w)
	if err != nil {
		return err
	}

	err = p.destPubkey.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (p *SigChain) Deserialization(r io.Reader) error {
	var err error
	err = p.DeserializationMetadata(r)
	if err != nil {
		return err
	}

	num, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	for i := 0; i < int(num); i++ {
		err = p.elems[i].Deserialization(r)
		return err
	}

	return nil
}

func (p *SigChain) DeserializationMetadata(r io.Reader) error {
	var err error
	dataSize, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	p.dataSize = dataSize

	err = p.dataHash.Deserialize(r)
	if err != nil {
		return err
	}

	err = p.srcPubkey.DeSerialize(r)
	if err != nil {
		return err
	}

	err = p.destPubkey.DeSerialize(r)
	if err != nil {
		return err
	}

	return nil
}

func (p *SigChainElem) Serialization(w io.Writer) error {
	var err error
	err = p.SerializationUnsigned(w)
	if err != nil {
		return err
	}
	err = serialization.WriteVarBytes(w, p.signature[:])
	if err != nil {
		return err
	}

	return nil
}

func (p *SigChainElem) SerializationUnsigned(w io.Writer) error {
	var err error
	err = p.pubkey.Serialize(w)
	if err != nil {
		return err
	}
	err = p.nextPubkey.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (p *SigChainElem) Deserialization(r io.Reader) error {
	var err error
	err = p.DeserializationUnsigned(r)
	if err != nil {
		return err
	}
	signature, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	p.signature = signature

	return nil
}

func (p *SigChainElem) DeserializationUnsigned(r io.Reader) error {
	var err error
	err = p.pubkey.DeSerialize(r)
	if err != nil {
		return err
	}

	err = p.nextPubkey.DeSerialize(r)
	if err != nil {
		return err
	}

	return nil
}
