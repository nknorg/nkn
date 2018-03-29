package sigchain

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"

	. "nkn-core/common"
	"nkn-core/common/serialization"
	"nkn-core/core/contract"
	"nkn-core/crypto"
)

type SigChain struct {
	elems []*SigChainElem
}

type SigChainElem struct {
	dataSize       uint32         // signed data size
	dataHash       *Uint256       // signed data hash
	nextScriptHash *Uint160       // next signer
	pubkey         *crypto.PubKey // current signer

	signature []byte // signature for signature chain element
}

// first relay node starts a new signature chain which consists a element.
func New(dataSize uint32, dataHash *Uint256, nextScriptHash *Uint160, pubKey *crypto.PubKey, signature []byte) *SigChain {
	var chain SigChain
	elem := &SigChainElem{
		dataSize:       dataSize,
		dataHash:       dataHash,
		nextScriptHash: nextScriptHash,
		pubkey:         pubKey,
		signature:      signature,
	}
	chain.elems = append(chain.elems, elem)

	return &chain
}

// Create a new signature chain element for relay node.
func NewElem(dataSize uint32, dataHash *Uint256, nextScriptHash *Uint160, pubKey *crypto.PubKey, signature []byte) *SigChainElem {
	return &SigChainElem{
		dataSize:       dataSize,
		dataHash:       dataHash,
		nextScriptHash: nextScriptHash,
		pubkey:         pubKey,
		signature:      signature,
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
	script, err := contract.CreateSignatureRedeemScript(elem.pubkey)
	if err != nil {
		return nil, err
	}
	scriptHash, err := ToCodeHash(script)
	if err != nil {
		return nil, err
	}
	// verify new element with last script hash in signature chain
	lastElem, err := p.LastSigElem()
	if err != nil {
		return nil, err
	}
	if scriptHash != *lastElem.nextScriptHash {
		return nil, errors.New("appending new element to signature chain error")
	}
	p.elems = append(p.elems, elem)

	return p, nil
}

// Verify returns result of signature chain verification.
func (p *SigChain) Verify() error {
	var err error
	var tmpPrevHash *Uint160
	var lastElemDataHash *Uint256

	lastElem, err := p.LastSigElem()
	if err != nil {
		return err
	}
	lastElemDataHash = lastElem.dataHash
	for _, e := range p.elems {
		// verify each element data hash
		if e.dataHash.CompareTo(*lastElemDataHash) != 0 {
			return errors.New("unmatch data hash in signature chain")
		}
		buff := bytes.NewBuffer(nil)
		// verify each element signature
		e.SerializationUnsigned(buff)
		dataHash := sha256.Sum256(buff.Bytes())
		err = crypto.Verify(*e.pubkey, dataHash[:], e.signature)
		if err != nil {
			return err
		}
		// verify hash matched or not
		if tmpPrevHash != nil {
			script, err := contract.CreateSignatureRedeemScript(e.pubkey)
			if err != nil {
				return err
			}
			scriptHash, err := ToCodeHash(script)
			if err != nil {
				return err
			}
			if tmpPrevHash.CompareTo(scriptHash) != 0 {
				return errors.New("invalid signature chain, unmatched with prev element next script hash")
			}
		}
		tmpPrevHash = e.nextScriptHash
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

// LastSigElem returns nextScriptHash filed of last element in signature chain.
func (p *SigChain) LastSigElem() (*SigChainElem, error) {
	if p == nil || len(p.elems) == 0 {
		return nil, errors.New("nil signature chain")
	}
	num := len(p.elems)

	return p.elems[num-1], nil
}

func (p *SigChain) Serialization(w io.Writer) error {
	var err error
	num := len(p.elems)
	serialization.WriteUint32(w, uint32(num))
	for i := 0; i < num; i++ {
		err = p.elems[i].Serialization(w)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *SigChain) Deserialization(r io.Reader) error {
	var err error
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
	err = serialization.WriteUint32(w, uint32(p.dataSize))
	if err != nil {
		return err
	}
	_, err = p.dataHash.Serialize(w)
	if err != nil {
		return err
	}
	_, err = p.nextScriptHash.Serialize(w)
	if err != nil {
		return err
	}
	err = p.pubkey.Serialize(w)
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
	dataSize, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	p.dataSize = dataSize

	err = p.dataHash.Deserialize(r)
	if err != nil {
		return err
	}

	err = p.nextScriptHash.Deserialize(r)
	if err != nil {
		return err
	}

	err = p.pubkey.DeSerialize(r)
	if err != nil {
		return err
	}

	return nil
}
