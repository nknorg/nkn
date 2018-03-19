package httpjsonrpc

import (
	. "nkn-core/common"
	. "nkn-core/core/transaction"
	"nkn-core/core/transaction/payload"
)

type PayloadInfo interface{}

//implement PayloadInfo define BookKeepingInfo
type BookKeepingInfo struct {
	Nonce  uint64
}

//implement PayloadInfo define DeployCodeInfo
type FunctionCodeInfo struct {
	Code           string
	ParameterTypes []int
	ReturnType    int
	CodeHash       string
}

type DeployCodeInfo struct {
	Code        *FunctionCodeInfo
	Name        string
	Version string
	Author      string
	Email       string
	Description string
	Language    int
	ProgramHash string
}

//implement PayloadInfo define TransferAssetInfo
type TransferAssetInfo struct {
}

func TransPayloadToHex(p Payload) PayloadInfo {
	switch object := p.(type) {
	case *payload.BookKeeping:
		obj := new(BookKeepingInfo)
		obj.Nonce = object.Nonce
		return obj
	case *payload.TransferAsset:
	case *payload.DeployCode:
		obj := new(DeployCodeInfo)
		obj.Code = new(FunctionCodeInfo)
		obj.Code.Code = ToHexString(object.Code.Code)
		var params []int
		for _, v := range object.Code.ParameterTypes {
			params = append(params, int(v))
		}
		obj.Code.ParameterTypes = params
		obj.Code.ReturnType = int(object.Code.ReturnType)
		codeHash := object.Code.CodeHash()
		obj.Code.CodeHash = ToHexString(codeHash.ToArrayReverse())
		obj.Name = object.Name
		obj.Version = object.CodeVersion
		obj.Author = object.Author
		obj.Email = object.Email
		obj.Description = object.Description
		obj.Language = int(object.Language)
		obj.ProgramHash = ToHexString(object.ProgramHash.ToArrayReverse())
		return obj
	}
	return nil
}
