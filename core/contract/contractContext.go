package contract

import (
	"errors"
	"math/big"
	"sort"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	_ "github.com/nknorg/nkn/errors"
	sig "github.com/nknorg/nkn/signature"
	"github.com/nknorg/nkn/types"
	"github.com/nknorg/nkn/util/log"
)

type ContractContext struct {
	Data          sig.SignableData
	ProgramHashes []Uint160
	Codes         [][]byte
	Parameters    [][][]byte

	MultiPubkeyPara [][]PubkeyParameter

	//temp index for multi sig
	tempParaIndex int
}

func NewContractContext(data sig.SignableData) *ContractContext {
	programHashes, _ := data.GetProgramHashes() //TODO: check error
	hashLen := len(programHashes)
	return &ContractContext{
		Data:            data,
		ProgramHashes:   programHashes,
		Codes:           make([][]byte, hashLen),
		Parameters:      make([][][]byte, hashLen),
		MultiPubkeyPara: make([][]PubkeyParameter, hashLen),
		tempParaIndex:   0,
	}
}

func (ctx *ContractContext) Add(contract *Contract, index int, parameter []byte) error {
	i := ctx.GetIndex(contract.ProgramHash)
	if i < 0 {
		return errors.New("Program Hash is not exist")
	}
	if ctx.Codes[i] == nil {
		ctx.Codes[i] = contract.Code
	}
	if ctx.Parameters[i] == nil {
		ctx.Parameters[i] = make([][]byte, len(contract.Parameters))
	}
	ctx.Parameters[i][index] = parameter
	return nil
}

func (ctx *ContractContext) AddContract(contract *Contract, pubkey *crypto.PubKey, parameter []byte) error {
	if contract.GetType() == MultiSigContract {
		index := ctx.GetIndex(contract.ProgramHash)
		if index < 0 {
			log.Error("The program hash is not exist.")
			return errors.New("The program hash is not exist.")
		}
		if ctx.Codes[index] == nil {
			ctx.Codes[index] = contract.Code
		}
		if ctx.Parameters[index] == nil {
			ctx.Parameters[index] = make([][]byte, len(contract.Parameters))
		}

		if err := ctx.Add(contract, ctx.tempParaIndex, parameter); err != nil {
			return err
		}

		ctx.tempParaIndex++

		//all paramenters added, sort the parameters
		if ctx.tempParaIndex == len(contract.Parameters) {
			ctx.tempParaIndex = 0
		}
	} else {
		//add non multi sig contract
		index := -1
		for i := 0; i < len(contract.Parameters); i++ {
			if contract.Parameters[i] == Signature {
				if index >= 0 {
					return errors.New("Contract Parameters are not supported.")
				} else {
					index = i
				}
			}
		}
		return ctx.Add(contract, index, parameter)
	}
	return nil
}

func (ctx *ContractContext) AddSignatureToMultiList(contractIndex int, contract *Contract, pubkey *crypto.PubKey, parameter []byte) error {
	if ctx.MultiPubkeyPara[contractIndex] == nil {
		ctx.MultiPubkeyPara[contractIndex] = make([]PubkeyParameter, len(contract.Parameters))
	}
	pk, err := pubkey.EncodePoint(true)
	if err != nil {
		return err
	}

	pubkeyPara := PubkeyParameter{
		PubKey:    BytesToHexString(pk),
		Parameter: BytesToHexString(parameter),
	}
	ctx.MultiPubkeyPara[contractIndex] = append(ctx.MultiPubkeyPara[contractIndex], pubkeyPara)

	return nil
}

func (ctx *ContractContext) AddMultiSignatures(index int, contract *Contract, pubkey *crypto.PubKey, parameter []byte) error {
	pkIndexs, err := ctx.ParseContractPubKeys(contract)
	if err != nil {
		return errors.New("Contract Parameters are not supported.")
	}

	paraIndexs := []ParameterIndex{}
	for _, pubkeyPara := range ctx.MultiPubkeyPara[index] {
		pubKeyBytes, err := HexStringToBytes(pubkeyPara.Parameter)
		if err != nil {
			return errors.New("Contract AddContract pubKeyBytes HexToBytes failed.")
		}

		paraIndex := ParameterIndex{
			Parameter: pubKeyBytes,
			Index:     pkIndexs[pubkeyPara.PubKey],
		}
		paraIndexs = append(paraIndexs, paraIndex)
	}

	//sort parameter by Index
	sort.Sort(sort.Reverse(ParameterIndexSlice(paraIndexs)))

	//generate sorted parameter list
	for i, paraIndex := range paraIndexs {
		if err := ctx.Add(contract, i, paraIndex.Parameter); err != nil {
			return err
		}
	}

	ctx.MultiPubkeyPara[index] = nil

	return nil
}

func (ctx *ContractContext) ParseContractPubKeys(contract *Contract) (map[string]int, error) {

	pubkeyIndex := make(map[string]int)

	Index := 0
	//parse contract's pubkeys
	i := 0
	switch contract.Code[i] {
	case 1:
		i += 2
		break
	case 2:
		i += 3
		break
	}
	for contract.Code[i] == 33 {
		i++
		//pubkey, err := crypto.DecodePoint(contract.Code[i:33])
		//if err != nil {
		//	return nil, errors.New("[Contract],AddContract DecodePoint failed.")
		//}

		//add to parameter index
		pubkeyIndex[BytesToHexString(contract.Code[i:33])] = Index

		i += 33
		Index++
	}

	return pubkeyIndex, nil
}

func (ctx *ContractContext) GetIndex(programHash Uint160) int {
	for i := 0; i < len(ctx.ProgramHashes); i++ {
		if ctx.ProgramHashes[i] == programHash {
			return i
		}
	}
	return -1
}

func (ctx *ContractContext) GetPrograms() []*types.Program {
	if !ctx.IsCompleted() {
		return nil
	}
	programs := make([]*types.Program, len(ctx.Parameters))

	for i := 0; i < len(ctx.Codes); i++ {
		sb := NewProgramBuilder()

		for _, parameter := range ctx.Parameters[i] {
			if len(parameter) <= 2 {
				sb.PushNumber(new(big.Int).SetBytes(parameter))
			} else {
				sb.PushData(parameter)
			}
		}
		programs[i] = &types.Program{
			Code:      ctx.Codes[i],
			Parameter: sb.ToArray(),
		}
	}
	return programs
}

func (ctx *ContractContext) IsCompleted() bool {
	for _, p := range ctx.Parameters {
		if p == nil {
			return false
		}

		for _, pp := range p {
			if pp == nil {
				return false
			}
		}
	}
	return true
}
