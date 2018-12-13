package payload

import (
	"github.com/nknorg/nkn/core/asset"
	"github.com/nknorg/nkn/crypto"
)

// define RegisterAssetInfo
type RegisterAssetInfo struct {
	Asset      *asset.Asset   `json:"asset"`
	Amount     string         `json:"amount"`
	Issuer     *crypto.PubKey `json:"issuer"`
	Controller string         `json:"controller"`
}

// define PrepaidInfo
type PrepaidInfo struct {
	Asset  string `json:"asset"`
	Amount string `json:"amount"`
	Rates  string `json:"rates"`
}

type WithdrawInfo struct {
	ProgramHash string `json:"programHash"`
}

type CommitInfo struct {
	SigChain  string `json:"sigChain"`
	Submitter string `json:"submitter"`
}

type RegisterNameInfo struct {
	Registrant string `json:"registrant"`
	Name       string `json:"name"`
}

type DeleteNameInfo struct {
	Registrant string `json:"registrant"`
	Name       string `json:"name"`
}

type SubscribeInfo struct {
	Subscriber string `json:"subscriber"`
	Identifier string `json:"identifier"`
	Topic      string `json:"topic"`
	Bucket     uint32 `json:"bucket"`
	Duration   uint32 `json:"duration"`
}