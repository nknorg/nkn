package contract

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/pb"
)

type ContractParameterType byte

const (
	Signature ContractParameterType = 0
	CHECKSIG  byte                  = 0xAC
)

//Contract address is the hash of contract program .
//which be used to control asset or indicate the smart contract address �?

//Contract include the program codes with parameters which can be executed on specific evnrioment
type Contract struct {

	//the contract program code,which will be run on VM or specific envrionment
	Code []byte

	//the Contract Parameter type list
	// describe the number of contract program parameters and the parameter type
	Parameters []ContractParameterType

	//The program hash as contract address
	ProgramHash Uint160

	//owner's pubkey hash indicate the owner of contract
	OwnerPubkeyHash Uint160
}

func (c *Contract) Deserialize(r io.Reader) error {
	code, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	c.Code = code

	p, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	c.Parameters = ByteToContractParameterType(p)

	err = c.ProgramHash.Deserialize(r)
	if err != nil {
		return err
	}

	return nil
}

func (c *Contract) Serialize(w io.Writer) error {
	err := serialization.WriteVarBytes(w, c.Code)
	if err != nil {
		return err
	}
	err = serialization.WriteVarBytes(w, ContractParameterTypeToByte(c.Parameters))
	if err != nil {
		return err
	}
	_, err = c.ProgramHash.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (c *Contract) ToArray() []byte {
	w := new(bytes.Buffer)
	c.Serialize(w)

	return w.Bytes()
}

func ContractParameterTypeToByte(c []ContractParameterType) []byte {
	b := make([]byte, len(c))

	for i := 0; i < len(c); i++ {
		b[i] = byte(c[i])
	}

	return b
}

func ByteToContractParameterType(b []byte) []ContractParameterType {
	c := make([]ContractParameterType, len(b))

	for i := 0; i < len(b); i++ {
		c[i] = ContractParameterType(b[i])
	}

	return c
}

//create a Single Singature contract for owner
func CreateSignatureContract(ownerPubKey *crypto.PubKey) (*Contract, error) {
	temp := ownerPubKey.EncodePoint()
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

//CODE: len(publickey) + publickey + CHECKSIG
func CreateSignatureRedeemScript(pubkey *crypto.PubKey) ([]byte, error) {
	encodedPublicKey := pubkey.EncodePoint()

	code := bytes.NewBuffer(nil)
	code.WriteByte(byte(len(encodedPublicKey)))
	code.Write(encodedPublicKey)
	code.WriteByte(byte(CHECKSIG))

	return code.Bytes(), nil
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

//CODE: len(publickey) + publickey + CHECKSIG
//--------------------------------------------
//Size:      1             32            1
func GetPublicKeyFromCode(code []byte) ([]byte, error) {
	if len(code) != 34 {
		return nil, fmt.Errorf("code length error, need 34, but got %v", len(code))
	}

	if code[0] != 32 && code[33] != 0xAC {
		return nil, fmt.Errorf("code format error, need code[0]=32, code[33]=0xac, but got %v and %x", code[0], code[33])
	}

	return code[1:33], nil
}

//Parameter: len(signature) + signature
//--------------------------------------------
//Size:          1             64
func GetSignatureFromParameter(parameter []byte) ([]byte, error) {
	if len(parameter) != 65 {
		return nil, fmt.Errorf("parameter length error, need 65,but got %v", len(parameter))
	}

	if parameter[0] != 64 {
		return nil, fmt.Errorf("parameter format error, need parameter[0]=64, bug got %v", parameter[0])
	}

	return parameter[1:], nil
}

//Parameter: len(signature) + signature
func (c *Contract) NewProgram(signature []byte) *pb.Program {
	size := len(signature)
	parameter := append([]byte{byte(size)}, signature...)

	return &pb.Program{
		Code:      c.Code,
		Parameter: parameter,
	}
}
