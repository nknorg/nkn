package httpjsonrpc

import (
	. "nkn-core/common"
	"nkn-core/core/asset"
	. "nkn-core/core/transaction"
	"nkn-core/core/transaction/payload"
)

type PayloadInfo interface{}

//implement PayloadInfo define BookKeepingInfo
type BookKeepingInfo struct {
	Nonce  uint64
	Issuer IssuerInfo
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

//implement PayloadInfo define IssueAssetInfo
type IssueAssetInfo struct {
}

type IssuerInfo struct {
	X, Y string
}

//implement PayloadInfo define RegisterAssetInfo
type RegisterAssetInfo struct {
	Asset      *asset.Asset
	Amount     Fixed64
	Issuer     IssuerInfo
	Controller string
}

//implement PayloadInfo define TransferAssetInfo
type TransferAssetInfo struct {
}

//implement PayloadInfo define BookKeepingInfo
type PrepaidInfo struct {
	Amount string
	Rates string
}

type BookkeeperInfo struct {
	PubKey     string
	Action     string
	Issuer     IssuerInfo
	Controller string
}


func TransPayloadToHex(p Payload) PayloadInfo {
	switch object := p.(type) {
	case *payload.BookKeeping:
		obj := new(BookKeepingInfo)
		obj.Nonce = object.Nonce
		return obj
	case *payload.BookKeeper:
		obj := new(BookkeeperInfo)
		encodedPubKey, _ := object.PubKey.EncodePoint(true)
		obj.PubKey = BytesToHexString(encodedPubKey)
		if object.Action == payload.BookKeeperAction_ADD {
			obj.Action = "add"
		} else if object.Action == payload.BookKeeperAction_SUB {
			obj.Action = "sub"
		} else {
			obj.Action = "nil"
		}
		obj.Issuer.X = object.Issuer.X.String()
		obj.Issuer.Y = object.Issuer.Y.String()

		return obj
	case *payload.IssueAsset:
	case *payload.TransferAsset:
	case *payload.Prepaid:
		obj := new(PrepaidInfo)
		obj.Amount = object.Amount.String()
		obj.Rates = object.Rates.String()
		return obj
	case *payload.DeployCode:
		obj := new(DeployCodeInfo)
		obj.Code = new(FunctionCodeInfo)
		obj.Code.Code = BytesToHexString(object.Code.Code)
		var params []int
		for _, v := range object.Code.ParameterTypes {
			params = append(params, int(v))
		}
		obj.Code.ParameterTypes = params
		obj.Code.ReturnType = int(object.Code.ReturnType)
		codeHash := object.Code.CodeHash()
		obj.Code.CodeHash = BytesToHexString(codeHash.ToArrayReverse())
		obj.Name = object.Name
		obj.Version = object.CodeVersion
		obj.Author = object.Author
		obj.Email = object.Email
		obj.Description = object.Description
		obj.Language = int(object.Language)
		obj.ProgramHash = BytesToHexString(object.ProgramHash.ToArrayReverse())
		return obj
	case *payload.RegisterAsset:
		obj := new(RegisterAssetInfo)
		obj.Asset = object.Asset
		obj.Amount = object.Amount
		obj.Issuer.X = object.Issuer.X.String()
		obj.Issuer.Y = object.Issuer.Y.String()
		obj.Controller = BytesToHexString(object.Controller.ToArray())
		return obj
	}
	return nil
}
