package httpjson

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	. "github.com/nknorg/nkn/common"
	. "github.com/nknorg/nkn/core/transaction"
	tx "github.com/nknorg/nkn/core/transaction"
)

type TxAttributeInfo struct {
	Usage TransactionAttributeUsage
	Data  string
}

type UTXOTxInputInfo struct {
	ReferTxID          string
	ReferTxOutputIndex uint16
}

type BalanceTxInputInfo struct {
	AssetID     string
	Value       string
	ProgramHash string
}

type TxoutputInfo struct {
	AssetID string
	Value   string
	Address string
}

type TxoutputMap struct {
	Key   Uint256
	Txout []TxoutputInfo
}

type AmountMap struct {
	Key   Uint256
	Value Fixed64
}

type ProgramInfo struct {
	Code      string
	Parameter string
}

type Transactions struct {
	TxType         TransactionType
	PayloadVersion byte
	Payload        PayloadInfo
	Attributes     []TxAttributeInfo
	UTXOInputs     []UTXOTxInputInfo
	BalanceInputs  []BalanceTxInputInfo
	Outputs        []TxoutputInfo
	Programs       []ProgramInfo

	AssetOutputs      []TxoutputMap
	AssetInputAmount  []AmountMap
	AssetOutputAmount []AmountMap

	Hash string
}

type BlockHead struct {
	Version          uint32
	PrevBlockHash    string
	TransactionsRoot string
	Timestamp        uint32
	Height           uint32
	ConsensusData    uint64
	NextBookKeeper   string
	Program          ProgramInfo

	Hash string
}

type BlockInfo struct {
	Hash         string
	BlockData    *BlockHead
	Transactions []*Transactions
}

type TxInfo struct {
	Hash string
	Hex  string
	Tx   *Transactions
}

type TxoutInfo struct {
	High  uint32
	Low   uint32
	Txout tx.TxOutput
}

type NodeInfo struct {
	State    uint   // node status
	Port     uint16 // The nodes's port
	ID       uint64 // The nodes's id
	Time     int64
	Version  uint32 // The network protocol the node used
	Services uint64 // The services the node supplied
	Relay    bool   // The relay capability of the node (merge into capbility flag)
	Height   uint64 // The node latest block height
	TxnCnt   uint64 // The transactions be transmit by this node
	RxTxnCnt uint64 // The transaction received by this node
}

type ConsensusInfo struct {
	// TODO
}

func responsePacking(result interface{}) map[string]interface{} {
	resp := map[string]interface{}{
		"result": result,
	}
	return resp
}

// Call sends RPC request to server
func Call(address string, method string, id interface{}, params []interface{}) ([]byte, error) {
	data, err := json.Marshal(map[string]interface{}{
		"method": method,
		"id":     id,
		"params": params,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Marshal JSON request: %v\n", err)
		return nil, err
	}
	resp, err := http.Post(address, "application/json", strings.NewReader(string(data)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "POST request: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GET response: %v\n", err)
		return nil, err
	}

	return body, nil
}
