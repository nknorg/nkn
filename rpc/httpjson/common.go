package httpjson

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
)

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

func getBlockTransactions(block *ledger.Block) interface{} {
	trans := make([]string, len(block.Transactions))
	for i := 0; i < len(block.Transactions); i++ {
		h := block.Transactions[i].Hash()
		trans[i] = common.BytesToHexString(h.ToArrayReverse())
	}
	hash := block.Hash()
	type BlockTransactions struct {
		Hash         string
		Height       uint32
		Transactions []string
	}
	b := BlockTransactions{
		Hash:         common.BytesToHexString(hash.ToArrayReverse()),
		Height:       block.Header.Height,
		Transactions: trans,
	}
	return b
}
