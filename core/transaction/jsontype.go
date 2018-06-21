package transaction

import "github.com/nknorg/nkn/core/contract/program"

type TxnInputInfo struct {
	ReferTxID          string `json:"referTxID"`
	ReferTxOutputIndex uint16 `json:"referTxOutputIndex"`
}

type TxnOutputInfo struct {
	AssetID string `json:"assetID"`
	Value   string `json:"value"`
	Address string `json:"address"`
}

type TxnAttributeInfo struct {
	Usage TransactionAttributeUsage `json:"usage"`
	Data  string                    `json:"data"`
}

type TransactionInfo struct {
	TxType         TransactionType       `json:"txType"`
	PayloadVersion byte                  `json:"payloadVersion"`
	Payload        interface{}           `json:"payload"`
	Attributes     []TxnAttributeInfo    `json:"attributes"`
	Inputs         []TxnInputInfo        `json:"inputs"`
	Outputs        []TxnOutputInfo       `json:"outputs"`
	Programs       []program.ProgramInfo `json:"programs"`
	Hash           string                `json:"hash"`
}
