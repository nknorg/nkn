package signature

import (
	"errors"
	"fmt"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
)

func GetPublicKeyFromCode(code []byte) ([]byte, error) {
	if len(code) != 34 {
		return nil, errors.New("code length error")
	}

	if code[0] != 32 && code[33] != 0xAC {
		return nil, errors.New("code format err")
	}

	return code[1:33], nil
}

func GetSignatureFromParameter(parameter []byte) ([]byte, error) {
	if len(parameter) != 65 {
		return nil, errors.New("parameter length error")
	}

	if parameter[0] != 64 {
		return nil, errors.New("parameter format err")
	}

	return parameter[1:], nil
}

func VerifySignableData(signableData SignableData) error {

	hashes, err := signableData.GetProgramHashes()
	if err != nil {
		return err
	}

	programs := signableData.GetPrograms()
	if len(hashes) != len(programs) {
		return fmt.Errorf("the number of data hashes %d is different with number of programs %d", len(hashes), len(programs))
	}

	programs = signableData.GetPrograms()
	for i := 0; i < len(programs); i++ {
		temp, _ := ToCodeHash(programs[i].Code)
		if hashes[i] != temp {
			return fmt.Errorf("The data hashes %v is different with corresponding program code %v", hashes[i], temp)
		}

		pk, err := GetPublicKeyFromCode(programs[i].Code)
		if err != nil {
			return err
		}

		pubkey, err := crypto.NewPubKeyFromBytes(pk)
		if err != nil {
			return err
		}

		signature, err := GetSignatureFromParameter(programs[i].Parameter)
		if err != nil {
			return err
		}

		_, err = VerifySignature(signableData, pubkey, signature)
		if err != nil {
			return err
		}
	}

	return nil
}

func VerifySignature(signableData SignableData, pubkey *crypto.PubKey, signature []byte) (bool, error) {
	err := crypto.Verify(*pubkey, GetHashForSigning(signableData), signature)
	if err != nil {
		return false, fmt.Errorf("[Validation], VerifySignature failed: %v", err)
	} else {
		return true, nil
	}
}
