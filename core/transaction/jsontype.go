package transaction

import "github.com/nknorg/nkn/core/contract/program"

type UTXOTxInputInfo struct {
	ReferTxID          string `json:"referTxID"`
	ReferTxOutputIndex uint16 `json:"referTxOutputIndex"`
}

type TxOutputInfo struct {
	AssetID string `json:"assetID"`
	Value   string `json:"value"`
	Address string `json:"address"`
}

type TxAttributeInfo struct {
	Usage TransactionAttributeUsage `json:"usage"`
	Data  string                    `json:"data"`
}

type TransactionInfo struct {
	TxType         TransactionType       `json:"txType"`
	PayloadVersion byte                  `json:"payloadVersion"`
	Payload        interface{}           `json:"payload"`
	Attributes     []TxAttributeInfo     `json:"attributes"`
	UTXOInputs     []UTXOTxInputInfo     `json:"utxoInputs"`
	Outputs        []TxOutputInfo        `json:"outputs"`
	Programs       []program.ProgramInfo `json:"programs"`
	Hash           string                `json:"hash"`
}
