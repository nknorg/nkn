package httpjson

import (
	"bytes"
	"math"

	"github.com/golang/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/net/chord"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/rpc/common"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/log"
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
	if len(params) < 1 {
		return RpcResultNil
	}

	var sigChain []byte
	var err error
	switch params[0].(type) {
	case string:
		str := params[0].(string)
		sigChain, err = HexStringToBytes(str)
		if err != nil {
			return RpcResultInvalidParameter
		}
	default:
		return RpcResultInvalidParameter
	}

	if s.wallet == nil {
		return RpcResult("open wallet first")
	}

	txn, err := MakeCommitTransaction(s.wallet, sigChain)
	if err != nil {
		return RpcResultInternalError
	}

	if errCode := s.VerifyAndSendTx(txn); errCode != ErrNoError {
		return RpcResultInvalidTransaction
	}

	txHash := txn.Hash()
	return RpcResult(BytesToHexString(txHash.ToArrayReverse()))
}

func sigchaintest(s *RPCServer, params []interface{}) map[string]interface{} {
	if s.wallet == nil {
		return RpcResult("open wallet first")
	}
	account, err := s.wallet.GetDefaultAccount()
	if err != nil {
		return RpcResultNil
	}
	dataHash := Uint256{}
	currentHeight := ledger.DefaultLedger.Store.GetHeight()
	blockHash, err := ledger.DefaultLedger.Store.GetBlockHash(currentHeight - 1)
	if err != nil {
		return RpcResultInternalError
	}
	srcID := s.node.GetChordAddr()
	encodedPublickKey, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return RpcResultInternalError
	}
	sigChain, err := por.NewSigChain(account, 1, dataHash[:], blockHash[:], srcID, encodedPublickKey, encodedPublickKey)
	if err != nil {
		return RpcResultInternalError
	}
	if err := sigChain.Sign(encodedPublickKey, account); err != nil {
		return RpcResultInternalError
	}
	if err := sigChain.Sign(encodedPublickKey, account); err != nil {
		return RpcResultInternalError
	}
	buf, err := proto.Marshal(sigChain)
	txn, err := MakeCommitTransaction(s.wallet, buf)
	if err != nil {
		return RpcResultInternalError
	}

	if errCode := s.VerifyAndSendTx(txn); errCode != ErrNoError {
		return RpcResultInvalidTransaction
	}

	txHash := txn.Hash()
	return RpcResult(BytesToHexString(txHash.ToArrayReverse()))
}

func getWsAddr(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 1 {
		return RpcResultNil
	}
	switch params[0].(type) {
	case string:
		clientID, _, err := address.ParseClientAddress(params[0].(string))
		ring := chord.GetRing()
		if ring == nil {
			log.Error("Empty ring")
			return RpcResultInternalError
		}
		vnode, err := ring.GetPredecessor(clientID)
		if err != nil {
			log.Error("Cannot get predecessor")
			return RpcResultInternalError
		}
		addr, err := vnode.HttpWsAddr()
		if err != nil {
			log.Error("Cannot get websocket address")
			return RpcResultInternalError
		}
		return RpcResult(addr)
	default:
		return RpcResultInvalidParameter
	}
}

func getTotalIssued(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 1 {
		return RpcResultNil
	}

	assetid, ok := params[0].(string)
	if !ok {
		return RpcResultInvalidParameter
	}

	var assetHash Uint256
	bys, err := HexStringToBytesReverse(assetid)
	if err != nil {
		return RpcResultInvalidParameter
	}

	if err := assetHash.Deserialize(bytes.NewReader(bys)); err != nil {
		return RpcResultInvalidParameter
	}

	amount, err := ledger.DefaultLedger.Store.GetQuantityIssued(assetHash)
	if err != nil {
		return RpcResultInvalidParameter
	}

	val := float64(amount) / math.Pow(10, 8)
	return RpcResult(val)
}

func getAssetByHash(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 1 {
		return RpcResultNil
	}

	str, ok := params[0].(string)
	if !ok {
		return RpcResultInvalidParameter
	}

	hex, err := HexStringToBytesReverse(str)
	if err != nil {
		return RpcResultInvalidParameter
	}

	var hash Uint256
	err = hash.Deserialize(bytes.NewReader(hex))
	if err != nil {
		return RpcResultInvalidParameter
	}

	asset, err := ledger.DefaultLedger.Store.GetAsset(hash)
	if err != nil {
		return RpcResultInvalidParameter
	}

	return RpcResult(asset)
}

func getBalanceByAddr(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 1 {
		return RpcResultNil
	}

	addr, ok := params[0].(string)
	if !ok {
		return RpcResultInvalidParameter
	}

	var programHash Uint160
	programHash, err := ToScriptHash(addr)
	if err != nil {
		return RpcResultInvalidParameter
	}

	unspends, err := ledger.DefaultLedger.Store.GetUnspentsFromProgramHash(programHash)
	var balance Fixed64 = 0
	for _, u := range unspends {
		for _, v := range u {
			balance = balance + v.Value
		}
	}

	val := float64(balance) / math.Pow(10, 8)
	return RpcResult(val)
}

func getBalanceByAsset(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 2 {
		return RpcResultNil
	}

	addr, ok := params[0].(string)
	assetid, k := params[1].(string)
	if !ok || !k {
		return RpcResultInvalidParameter
	}

	var programHash Uint160
	programHash, err := ToScriptHash(addr)
	if err != nil {
		return RpcResultInvalidParameter
	}

	unspends, err := ledger.DefaultLedger.Store.GetUnspentsFromProgramHash(programHash)
	var balance Fixed64 = 0
	for k, u := range unspends {
		assid := BytesToHexString(k.ToArrayReverse())
		for _, v := range u {
			if assetid == assid {
				balance = balance + v.Value
			}
		}
	}

	val := float64(balance) / math.Pow(10, 8)
	return RpcResult(val)
}

func getUnspendOutput(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 2 {
		return RpcResultNil
	}

	addr, ok := params[0].(string)
	assetid, k := params[1].(string)
	if !ok || !k {
		return RpcResultInvalidParameter
	}

	var programHash Uint160
	var assetHash Uint256
	programHash, err := ToScriptHash(addr)
	if err != nil {
		return RpcResultInvalidParameter
	}

	bys, err := HexStringToBytesReverse(assetid)
	if err != nil {
		return RpcResultInvalidParameter
	}

	if err := assetHash.Deserialize(bytes.NewReader(bys)); err != nil {
		return RpcResultInvalidParameter
	}

	type UTXOUnspentInfo struct {
		Txid  string
		Index uint32
		Value float64
	}

	infos, err := ledger.DefaultLedger.Store.GetUnspentFromProgramHash(programHash, assetHash)
	if err != nil {
		return RpcResultInvalidParameter
	}

	var UTXOoutputs []UTXOUnspentInfo
	for _, v := range infos {
		val := float64(v.Value) / math.Pow(10, 8)
		UTXOoutputs = append(UTXOoutputs, UTXOUnspentInfo{Txid: BytesToHexString(v.Txid.ToArrayReverse()), Index: v.Index, Value: val})
	}

	return RpcResult(UTXOoutputs)
}

func getUnspends(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 1 {
		return RpcResultNil
	}

	addr, ok := params[0].(string)
	if !ok {
		return RpcResultInvalidParameter
	}
	var programHash Uint160

	programHash, err := ToScriptHash(addr)
	if err != nil {
		return RpcResultInvalidParameter
	}

	type UTXOUnspentInfo struct {
		Txid  string
		Index uint32
		Value float64
	}
	type Result struct {
		AssetId   string
		AssetName string
		Utxo      []UTXOUnspentInfo
	}

	var results []Result
	unspends, err := ledger.DefaultLedger.Store.GetUnspentsFromProgramHash(programHash)

	for k, u := range unspends {
		assetid := BytesToHexString(k.ToArrayReverse())
		asset, err := ledger.DefaultLedger.Store.GetAsset(k)
		if err != nil {
			return RpcResultInvalidParameter
		}

		var unspendsInfo []UTXOUnspentInfo
		for _, v := range u {
			val := float64(v.Value) / math.Pow(10, 8)
			unspendsInfo = append(unspendsInfo, UTXOUnspentInfo{BytesToHexString(v.Txid.ToArrayReverse()), v.Index, val})
		}

		results = append(results, Result{assetid, asset.Name, unspendsInfo})
	}

	return RpcResult(results)
}
