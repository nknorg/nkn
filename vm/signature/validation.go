package signature

import (
	"errors"
	"fmt"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/vm"
	"github.com/nknorg/nkn/vm/interfaces"
)

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
		//execute program on VM
		var cryptos interfaces.ICrypto
		cryptos = new(vm.ECDsaCrypto)
		se := vm.NewExecutionEngine(signableData, cryptos, nil, nil, Fixed64(0))
		se.LoadCode(programs[i].Code, false)
		se.LoadCode(programs[i].Parameter, true)
		err := se.Execute()

		if err != nil {
			return err
		}

		if se.GetState() != vm.HALT {
			return errors.New("[VM] Finish State not equal to HALT.")
		}

		if se.GetEvaluationStack().Count() != 1 {
			return errors.New("[VM] Execute Engine Stack Count Error.")
		}

		flag := se.GetExecuteResult()
		if !flag {
			return errors.New("[VM] Check Sig FALSE.")
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
