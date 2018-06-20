package httpjson

import (
	"github.com/nknorg/nkn/rpc/common"
)

var initialRPCHandlers = map[string]funcHandler{
	"getlatestblockhash":   getLatestBlockHash,
	"getblock":             getBlock,
	"getblockcount":        getBlockCount,
	"getblockhash":         getBlockHash,
	"getlatestblockheight": getLatestBlockHeight,
	"getblocktxsbyheight":  getBlockTxsByHeight,
	"getconnectioncount":   getConnectionCount,
	"getrawmempool":        getRawMemPool,
	"gettransaction":       getTransaction,
	"sendrawtransaction":   sendRawTransaction,
	"getwsaddr":            getWsAddr,
	"getversion":           getVersion,
	"getneighbor":          getNeighbor,
	"getnodestate":         getNodeState,
	"getchordringinfo":     getChordRingInfo,
	//"getbalance":           getBalance,
	//"setdebuginfo":         setDebugInfo,
	//"sendtoaddress":        sendToAddress,
	//"registasset":          registAsset,
	//"issueasset":           issueAsset,
	//"prepaidasset":         prepaidAsset,
	//"withdrawasset":        withdrawAsset,
	//"commitpor":            commitPor,
	//"sigchaintest":         sigchaintest,
	//"gettotalissued":       getTotalIssued,
	//"getassetbyhash":       getAssetByHash,
	//"getbalancebyaddr":     getBalanceByAddr,
	//"getbalancebyasset":    getBalanceByAsset,
	//"getunspendoutput":     getUnspendOutput,
	//"getunspends":          getUnspends,
}

func processUnify(cmd string, s *RPCServer, params []interface{}) map[string]interface{} {
	handler := common.InitialAPIHandlers[cmd]
	if !handler.IsAccessableByJsonrpc() {
		return RpcResultNil
	}
	resp, err := handler.Handler(s, params)
	if err != common.SUCCESS {
		return RpcResultInvalidParameter
	}
	return resp
}

func getLatestBlockHash(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getlatestblockhash", s, params)
}

// Input JSON string examples for getblock method as following:
//   {"jsonrpc": "2.0", "method": "getblock", "params": [1], "id": 0}
//   {"jsonrpc": "2.0", "method": "getblock", "params": ["aabbcc.."], "id": 0}
func getBlock(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getblock", s, params)
}

func getBlockCount(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getblockcount", s, params)
}

func getChordRingInfo(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getchordringinfo", s, params)
}

func getLatestBlockHeight(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getlatestblockheight", s, params)
}

// A JSON example for getblockhash method as following:
//   {"jsonrpc": "2.0", "method": "getblockhash", "params": [1], "id": 0}
func getBlockHash(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getblockhash", s, params)
}

// Input JSON string examples for getblocktxsbyheight method as following:
//   {"jsonrpc": "2.0", "method": "getblocktxsbyheight", "params": [1], "id": 0}
func getBlockTxsByHeight(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getblocktxsbyheight", s, params)
}

func getConnectionCount(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getconnectioncount", s, params)
}

func getRawMemPool(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getrawmempool", s, params)
}

// A JSON example for gettransaction method as following:
//   {"jsonrpc": "2.0", "method": "gettransaction", "params": ["transactioin hash in hex"], "id": 0}
func getTransaction(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("gettransaction", s, params)
}

// A JSON example for sendrawtransaction method as following:
//   {"jsonrpc": "2.0", "method": "sendrawtransaction", "params": ["raw transactioin in hex"], "id": 0}
func sendRawTransaction(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("sendrawtransaction", s, params)
}

func getNeighbor(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getneighbor", s, params)
}

func getNodeState(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getnodestate", s, params)
}

func setDebugInfo(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("setdebuginfo", s, params)
}

func getVersion(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getversion", s, params)
}

func getBalance(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getbalance", s, params)
}

func registAsset(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("registasset", s, params)
}

func issueAsset(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("issueasset", s, params)
}

func sendToAddress(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("sendtoaddress", s, params)
}

func prepaidAsset(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("prepaidasset", s, params)
}

func withdrawAsset(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("withdrawasset", s, params)
}

func commitPor(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("commitpor", s, params)
}

func sigchaintest(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("sigchaintest", s, params)
}

func getWsAddr(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getwsaddr", s, params)
}

func getTotalIssued(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("gettotalissued", s, params)
}

func getAssetByHash(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getassetbyhash", s, params)
}

func getBalanceByAddr(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getbalancebyaddr", s, params)
}

func getBalanceByAsset(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getbalancebyasset", s, params)
}

func getUnspendOutput(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getunspendoutput", s, params)
}

func getUnspends(s *RPCServer, params []interface{}) map[string]interface{} {
	return processUnify("getunspends", s, params)
}
