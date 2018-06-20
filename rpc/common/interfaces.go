package common

import (
	"bytes"
	"encoding/json"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/net/chord"
)

const (
	BIT_JSONRPC   byte = 1
	BIT_RESTFUL   byte = 2
	BIT_WEBSOCKET byte = 4
)

type handler func(Serverer, []interface{}) (map[string]interface{}, ErrCode)

type APIHandler struct {
	Handler    handler
	AccessCtrl byte
}

func (ah *APIHandler) IsAccessableByJsonrpc() bool {
	if ah.AccessCtrl&BIT_JSONRPC != BIT_JSONRPC {
		return false
	}

	return true
}

func (ah *APIHandler) IsAccessableByRestful() bool {
	if ah.AccessCtrl&BIT_RESTFUL != BIT_RESTFUL {
		return false
	}

	return true
}

func (ah *APIHandler) IsAccessableByWebsocket() bool {
	if ah.AccessCtrl&BIT_WEBSOCKET != BIT_WEBSOCKET {
		return false
	}

	return true
}

func getLatestBlockHash(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	hash := ledger.DefaultLedger.Blockchain.CurrentBlockHash()
	resp["result"] = common.BytesToHexString(hash.ToArrayReverse())

	return resp, SUCCESS
}

func getBlock(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 1 {
		return nil, INTERNAL_ERROR
	}

	var hash common.Uint256
	switch (params[0]).(type) {
	case float64: // block height
		index := uint32(params[0].(float64))
		var err error
		if hash, err = ledger.DefaultLedger.Store.GetBlockHash(index); err != nil {
			return nil, INTERNAL_ERROR
		}
	case string: // block hash
		str := params[0].(string)
		hex, err := common.HexStringToBytesReverse(str)
		if err != nil {
			return nil, INTERNAL_ERROR
		}
		if err := hash.Deserialize(bytes.NewReader(hex)); err != nil {
			return nil, INTERNAL_ERROR
		}
	default:
		return nil, INTERNAL_ERROR
	}

	block, err := ledger.DefaultLedger.Store.GetBlock(hash)
	if err != nil {
		return nil, INTERNAL_ERROR
	}
	block.Hash()

	var b interface{}
	info, _ := block.MarshalJson()
	json.Unmarshal(info, &b)
	resp["result"] = b

	return resp, SUCCESS
}

func getBlockCount(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	resp["result"] = ledger.DefaultLedger.Blockchain.BlockHeight + 1

	return resp, SUCCESS
}

func getChordRingInfo(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	resp["result"] = chord.GetRing()

	return resp, SUCCESS
}

func getLatestBlockHeight(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	resp["result"] = ledger.DefaultLedger.Blockchain.BlockHeight

	return resp, SUCCESS
}

var InitialAPIHandlers = map[string]APIHandler{
	"getlatestblockhash": {
		Handler:    getLatestBlockHash,
		AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET,
	},
	"getblock": {
		Handler:    getBlock,
		AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET,
	},
	"getblockcount": {
		Handler:    getBlockCount,
		AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET,
	},
	"getchordringinfo": {
		Handler:    getChordRingInfo,
		AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET,
	},
	"getlatestblockheight": {
		Handler:    getLatestBlockHeight,
		AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET,
	},
}
