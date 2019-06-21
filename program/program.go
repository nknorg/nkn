package program

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

type ProgramContextParameterType byte

const (
	Signature ProgramContextParameterType = 0
	CHECKSIG  byte                        = 0xAC
)

type ProgramContext struct {

	//the program code,which will be run on VM or specific envrionment
	Code []byte

	//the ProgramContext Parameter type list
	// describe the number of program parameters and the parameter type
	Parameters []ProgramContextParameterType

	//The program hash as address
	ProgramHash Uint160

	//owner's pubkey hash indicate the owner of program
	OwnerPubkeyHash Uint160
}

func (c *ProgramContext) Deserialize(r io.Reader) error {
	code, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	c.Code = code

	p, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	c.Parameters = ByteToProgramContextParameterType(p)

	err = c.ProgramHash.Deserialize(r)
	if err != nil {
		return err
	}

	return nil
}

func (c *ProgramContext) Serialize(w io.Writer) error {
	err := serialization.WriteVarBytes(w, c.Code)
	if err != nil {
		return err
	}
	err = serialization.WriteVarBytes(w, ProgramContextParameterTypeToByte(c.Parameters))
	if err != nil {
		return err
	}
	_, err = c.ProgramHash.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (c *ProgramContext) ToArray() []byte {
	w := new(bytes.Buffer)
	c.Serialize(w)

	return w.Bytes()
}

func ProgramContextParameterTypeToByte(c []ProgramContextParameterType) []byte {
	b := make([]byte, len(c))

	for i := 0; i < len(c); i++ {
		b[i] = byte(c[i])
	}

	return b
}

func ByteToProgramContextParameterType(b []byte) []ProgramContextParameterType {
	c := make([]ProgramContextParameterType, len(b))

	for i := 0; i < len(b); i++ {
		c[i] = ProgramContextParameterType(b[i])
	}

	return c
}

//create a Single Singature program context for owner
func CreateSignatureProgramContext(ownerPubKey *crypto.PubKey) (*ProgramContext, error) {
	temp := ownerPubKey.EncodePoint()
	code, err := CreateSignatureProgramCode(ownerPubKey)
	if err != nil {
		return nil, fmt.Errorf("[ProgramContext],CreateSignatureProgramContext failed: %v", err)
	}
	hash, err := ToCodeHash(temp)
	if err != nil {
		return nil, fmt.Errorf("[ProgramContext],CreateSignatureProgramContext failed: %v", err)
	}
	programHash, err := ToCodeHash(code)
	if err != nil {
		return nil, fmt.Errorf("[ProgramContext],CreateSignatureProgramContext failed: %v", err)
	}
	return &ProgramContext{
		Code:            code,
		Parameters:      []ProgramContextParameterType{Signature},
		ProgramHash:     programHash,
		OwnerPubkeyHash: hash,
	}, nil
}

//CODE: len(publickey) + publickey + CHECKSIG
func CreateSignatureProgramCode(pubkey *crypto.PubKey) ([]byte, error) {
	encodedPublicKey := pubkey.EncodePoint()

	code := bytes.NewBuffer(nil)
	code.WriteByte(byte(len(encodedPublicKey)))
	code.Write(encodedPublicKey)
	code.WriteByte(byte(CHECKSIG))

	return code.Bytes(), nil
}

func CreateProgramHash(pubkey *crypto.PubKey) (Uint160, error) {
	code, err := CreateSignatureProgramCode(pubkey)
	if err != nil {
		return Uint160{}, errors.New("CreateSignatureProgramCode failed")
	}
	programHash, err := ToCodeHash(code)
	if err != nil {
		return Uint160{}, errors.New("ToCodeHash failed")
	}

	return programHash, err
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
func (c *ProgramContext) NewProgram(signature []byte) *pb.Program {
	size := len(signature)
	parameter := append([]byte{byte(size)}, signature...)

	return &pb.Program{
		Code:      c.Code,
		Parameter: parameter,
	}
}
