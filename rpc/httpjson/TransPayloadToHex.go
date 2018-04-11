package httpjson

import (
	. "nkn/common"
	"nkn/core/asset"
	. "nkn/core/transaction"
	"nkn/core/transaction/payload"
)

type PayloadInfo interface{}

//implement PayloadInfo define BookKeepingInfo
type BookKeepingInfo struct {
	Nonce  uint64
	Issuer IssuerInfo
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

func TransPayloadToHex(p Payload) PayloadInfo {
	switch object := p.(type) {
	case *payload.BookKeeping:
		obj := new(BookKeepingInfo)
		obj.Nonce = object.Nonce
		return obj
	case *payload.IssueAsset:
	case *payload.TransferAsset:
	case *payload.Prepaid:
		obj := new(PrepaidInfo)
		obj.Amount = object.Amount.String()
		obj.Rates = object.Rates.String()
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
