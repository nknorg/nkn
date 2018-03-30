package sigchain

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"

	. "nkn-core/common"
	"nkn-core/common/log"
	"nkn-core/common/serialization"
	"nkn-core/crypto"
)

// for the first relay node
// 1. New : create a new Signature Chain
// 2. Sign: sign the first element
//
// for the next relay node
// 1. NewElem : Create a new Signature Chain Element
// 2. Sign: sign the element create above
// 3. Append: append new element to existed signature chain

// TODO fake private key for signing, use local wallet instead
var privateKeyStub = []byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
}

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
func New(dataSize uint32, dataHash *Uint256, srcPubkey *crypto.PubKey, destPubkey *crypto.PubKey, nextPubkey *crypto.PubKey) *SigChain {
	var chain SigChain
	chain.dataSize = dataSize
	chain.dataHash = dataHash
	chain.srcPubkey = srcPubkey
	chain.destPubkey = destPubkey
	firstElem := NewElem(srcPubkey, nextPubkey)
	chain.elems = append(chain.elems, firstElem)

	return &chain
}

// Create a new signature chain element for relay node.
func NewElem(pubKey *crypto.PubKey, nextPubkey *crypto.PubKey) *SigChainElem {
	return &SigChainElem{
		pubkey:     pubKey,
		nextPubkey: nextPubkey,
		signature:  nil,
	}
}

// Sign new created signature chain with local wallet.
func (p *SigChain) Sign(elem *SigChainElem) error {
	sigNum := p.SignatureNum()
	switch {
	case sigNum <= 0:
		return errors.New("uninitialized signature chain")
	case sigNum == 1:
		firstElem, err := p.FirstSigElem()
		if err != nil {
			return err
		}
		if firstElem.signature != nil {
			log.Warn("new signature chain resigned")
		}
		buff := bytes.NewBuffer(nil)
		if err := p.SerializationMetadata(buff); err != nil {
			return err
		}
		if err := firstElem.SerializationUnsigned(buff); err != nil {
			return err
		}
		hash := sha256.Sum256(buff.Bytes())
		signature, err := crypto.Sign(privateKeyStub, hash[:])
		if err != nil {
			return err
		}
		firstElem.signature = signature
	case sigNum > 1:
		return errors.New("sign signature chain error")
	}

	return nil
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
		serialization.WriteVarBytes(buff, prevHash[:])
		e.SerializationUnsigned(buff)
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

// SignatureNum returns element num in current signature chain
func (p *SigChain) SignatureNum() int {
	return len(p.elems)
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
	err = serialization.WriteUint32(w, p.dataSize)
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

// Sign new signature chain element with local wallet.
func (p *SigChainElem) Sign(sigchain *SigChain) error {
	var err error
	lastElem, err := sigchain.LastSigElem()
	if err != nil {
		return err
	}
	buff := bytes.NewBuffer(nil)
	err = serialization.WriteVarBytes(buff, lastElem.signature)
	if err != nil {
		return err
	}
	err = p.SerializationUnsigned(buff)
	if err != nil {
		return err
	}
	signature, err := crypto.Sign(privateKeyStub, buff.Bytes())
	if err != nil {
		return err
	}
	p.signature = signature

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
