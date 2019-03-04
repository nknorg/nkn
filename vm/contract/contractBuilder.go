package contract

import (
	"errors"
	"fmt"
	"math/big"
	"sort"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/vm"
)

//create a Single Singature contract for owner
func CreateSignatureContract(ownerPubKey *crypto.PubKey) (*Contract, error) {
	temp, err := ownerPubKey.EncodePoint(true)
	if err != nil {
		return nil, fmt.Errorf("[Contract],CreateSignatureContract failed: %v", err)
	}
	signatureRedeemScript, err := CreateSignatureRedeemScript(ownerPubKey)
	if err != nil {
		return nil, fmt.Errorf("[Contract],CreateSignatureContract failed: %v", err)
	}
	hash, err := ToCodeHash(temp)
	if err != nil {
		return nil, fmt.Errorf("[Contract],CreateSignatureContract failed: %v", err)
	}
	signatureRedeemScriptHashToCodeHash, err := ToCodeHash(signatureRedeemScript)
	if err != nil {
		return nil, fmt.Errorf("[Contract],CreateSignatureContract failed: %v", err)
	}
	return &Contract{
		Code:            signatureRedeemScript,
		Parameters:      []ContractParameterType{Signature},
		ProgramHash:     signatureRedeemScriptHashToCodeHash,
		OwnerPubkeyHash: hash,
	}, nil
}

func CreateSignatureRedeemScript(pubkey *crypto.PubKey) ([]byte, error) {
	temp, err := pubkey.EncodePoint(true)
	if err != nil {
		return nil, fmt.Errorf("[Contract],CreateSignatureRedeemScript failed: %v", err)
	}
	return CreateSignatureRedeemScriptWithEncodedPublicKey(temp)
}

func CreateSignatureRedeemScriptWithEncodedPublicKey(encodedPublicKey []byte) ([]byte, error) {
	sb := NewProgramBuilder()
	sb.PushData(encodedPublicKey)
	sb.AddOp(vm.CHECKSIG)
	return sb.ToArray(), nil
}

//create a Multi Singature contract for owner  。
func CreateMultiSigContract(publicKeyHash Uint160, m int, publicKeys []*crypto.PubKey) (*Contract, error) {

	params := make([]ContractParameterType, m)
	for i, _ := range params {
		params[i] = Signature
	}
	MultiSigRedeemScript, err := CreateMultiSigRedeemScript(m, publicKeys)
	if err != nil {
		return nil, fmt.Errorf("[Contract],CreateSignatureRedeemScript failed: %v", err)
	}
	signatureRedeemScriptHashToCodeHash, err := ToCodeHash(MultiSigRedeemScript)
	if err != nil {
		return nil, fmt.Errorf("[Contract],CreateSignatureContract failed: %v", err)
	}
	return &Contract{
		Code:            MultiSigRedeemScript,
		Parameters:      params,
		ProgramHash:     signatureRedeemScriptHashToCodeHash,
		OwnerPubkeyHash: publicKeyHash,
	}, nil
}

func CreateMultiSigRedeemScript(m int, pubkeys []*crypto.PubKey) ([]byte, error) {
	if !(m >= 1 && m <= len(pubkeys) && len(pubkeys) <= 24) {
		return nil, nil //TODO: add panic
	}

	sb := NewProgramBuilder()
	sb.PushNumber(big.NewInt(int64(m)))

	//sort pubkey
	sort.Sort(crypto.PubKeySlice(pubkeys))

	for _, pubkey := range pubkeys {
		temp, err := pubkey.EncodePoint(true)
		if err != nil {
			return nil, fmt.Errorf("[Contract],CreateSignatureContract failed: %v", err)
		}
		sb.PushData(temp)
	}

	sb.PushNumber(big.NewInt(int64(len(pubkeys))))
	sb.AddOp(vm.CHECKMULTISIG)
	return sb.ToArray(), nil
}

func CreateRedeemHash(pubkey *crypto.PubKey) (Uint160, error) {
	signatureRedeemScript, err := CreateSignatureRedeemScript(pubkey)
	if err != nil {
		return Uint160{}, errors.New("CreateSignatureRedeemScript failed")
	}
	RedeemHash, err := ToCodeHash(signatureRedeemScript)
	if err != nil {
		return Uint160{}, errors.New("ToCodeHash failed")
	}

	return RedeemHash, err
}
