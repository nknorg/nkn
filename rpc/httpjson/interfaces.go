package httpjson

import (
	"bytes"
	"encoding/json"
	"math"

	"github.com/golang/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	tx "github.com/nknorg/nkn/core/transaction"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/net/chord"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

var initialRPCHandlers = map[string]funcHandler{
	"getlatestblockhash":   getLatestBlockHash,
	"getblock":             getBlock,
	"getblockcount":        getBlockCount,
	"getblockhash":         getBlockHash,
	"getconnectioncount":   getConnectionCount,
	"getrawmempool":        getRawMemPool,
	"gettransaction":       getTransaction,
	"getwsaddr":            getWsAddr,
	"sendrawtransaction":   sendRawTransaction,
	"getversion":           getVersion,
	"getneighbor":          getNeighbor,
	"getnodestate":         getNodeState,
	"getbalance":           getBalance,
	"setdebuginfo":         setDebugInfo,
	"sendtoaddress":        sendToAddress,
	"registasset":          registAsset,
	"issueasset":           issueAsset,
	"prepaidasset":         prepaidAsset,
	"withdrawasset":        withdrawAsset,
	"commitpor":            commitPor,
	"getlatestblockheight": getLatestBlockHeight,
	"getblocktxsbyheight":  getBlockTxsByHeight,
	"getchordringinfo":     getChordRingInfo,
	"sigchaintest":         sigchaintest,
	"gettotalissued":       getTotalIssued,
	"getassetbyhash":       getAssetByHash,
}

func getLatestBlockHash(s *RPCServer, params []interface{}) map[string]interface{} {
	hash := ledger.DefaultLedger.Blockchain.CurrentBlockHash()
	return RpcResult(BytesToHexString(hash.ToArrayReverse()))
}

// Input JSON string examples for getblock method as following:
//   {"jsonrpc": "2.0", "method": "getblock", "params": [1], "id": 0}
//   {"jsonrpc": "2.0", "method": "getblock", "params": ["aabbcc.."], "id": 0}
func getBlock(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 1 {
		return RpcResultNil
	}
	var err error
	var hash Uint256
	switch (params[0]).(type) {
	// block height
	case float64:
		index := uint32(params[0].(float64))
		hash, err = ledger.DefaultLedger.Store.GetBlockHash(index)
		if err != nil {
			return RpcResultUnknownBlock
		}
	// block hash
	case string:
		str := params[0].(string)
		hex, err := HexStringToBytesReverse(str)
		if err != nil {
			return RpcResultInvalidParameter
		}
		if err := hash.Deserialize(bytes.NewReader(hex)); err != nil {
			return RpcResultInvalidTransaction
		}
	default:
		return RpcResultInvalidParameter
	}

	block, err := ledger.DefaultLedger.Store.GetBlock(hash)
	if err != nil {
		return RpcResultUnknownBlock
	}
	block.Hash()

	var b interface{}
	info, _ := block.MarshalJson()
	json.Unmarshal(info, &b)

	return RpcResult(b)
}

func getBlockCount(s *RPCServer, params []interface{}) map[string]interface{} {
	return RpcResult(ledger.DefaultLedger.Blockchain.BlockHeight + 1)
}

func getChordRingInfo(s *RPCServer, params []interface{}) map[string]interface{} {
	return RpcResult(chord.GetRing())
}

func getLatestBlockHeight(s *RPCServer, params []interface{}) map[string]interface{} {
	return RpcResult(ledger.DefaultLedger.Blockchain.BlockHeight)
}

// A JSON example for getblockhash method as following:
//   {"jsonrpc": "2.0", "method": "getblockhash", "params": [1], "id": 0}
func getBlockHash(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 1 {
		return RpcResultNil
	}
	switch params[0].(type) {
	case float64:
		height := uint32(params[0].(float64))
		hash, err := ledger.DefaultLedger.Store.GetBlockHash(height)
		if err != nil {
			return RpcResultUnknownBlock
		}
		return RpcResult(BytesToHexString(hash.ToArrayReverse()))
	default:
		return RpcResultInvalidParameter
	}
}

// Input JSON string examples for getblocktxsbyheight method as following:
//   {"jsonrpc": "2.0", "method": "getblocktxsbyheight", "params": [1], "id": 0}
func getBlockTxsByHeight(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 1 {
		return RpcResultNil
	}
	var err error

	if _, ok := params[0].(float64); !ok {
		return RpcResultInvalidParameter
	}
	index := uint32(params[0].(float64))
	hash, err := ledger.DefaultLedger.Store.GetBlockHash(index)
	if err != nil {
		return RpcResultUnknownBlock
	}

	block, err := ledger.DefaultLedger.Store.GetBlock(hash)
	if err != nil {
		return RpcResultUnknownBlock
	}

	txs := getBlockTransactions(block)

	return RpcResult(txs)
}

func getConnectionCount(s *RPCServer, params []interface{}) map[string]interface{} {
	return RpcResult(s.node.GetConnectionCnt())
}

func getRawMemPool(s *RPCServer, params []interface{}) map[string]interface{} {
	txs := []interface{}{}
	txpool := s.node.GetTxnPool()
	for _, t := range txpool.GetAllTransactions() {
		info, err := t.MarshalJson()
		if err != nil {
			return RpcResultInvalidTransaction
		}
		var x interface{}
		err = json.Unmarshal(info, &x)
		if err != nil {
			return RpcResultInvalidTransaction
		}
		txs = append(txs, x)
	}
	if len(txs) == 0 {
		return RpcResultNil
	}

	return RpcResult(txs)
}

// A JSON example for gettransaction method as following:
//   {"jsonrpc": "2.0", "method": "gettransaction", "params": ["transactioin hash in hex"], "id": 0}
func getTransaction(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 1 {
		return RpcResultNil
	}
	switch params[0].(type) {
	case string:
		str := params[0].(string)
		hex, err := HexStringToBytesReverse(str)
		if err != nil {
			return RpcResultInvalidParameter
		}
		var hash Uint256
		err = hash.Deserialize(bytes.NewReader(hex))
		if err != nil {
			return RpcResultInvalidTransaction
		}
		tx, err := ledger.DefaultLedger.Store.GetTransaction(hash)
		if err != nil {
			return RpcResultUnknownTransaction
		}

		tx.Hash()
		var tran interface{}
		info, _ := tx.MarshalJson()
		json.Unmarshal(info, &tran)

		return RpcResult(tran)
	default:
		return RpcResultInvalidParameter
	}
}

// A JSON example for sendrawtransaction method as following:
//   {"jsonrpc": "2.0", "method": "sendrawtransaction", "params": ["raw transactioin in hex"], "id": 0}
func sendRawTransaction(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 1 {
		return RpcResultNil
	}
	var hash Uint256
	switch params[0].(type) {
	case string:
		str := params[0].(string)
		hex, err := HexStringToBytes(str)
		if err != nil {
			return RpcResultInvalidParameter
		}
		var txn tx.Transaction
		if err := txn.Deserialize(bytes.NewReader(hex)); err != nil {
			return RpcResultInvalidTransaction
		}

		hash = txn.Hash()
		if errCode := s.VerifyAndSendTx(&txn); errCode != ErrNoError {
			return RpcResult(errCode.Error())
		}
	default:
		return RpcResultInvalidParameter
	}
	return RpcResult(BytesToHexString(hash.ToArrayReverse()))
}

func getTxout(s *RPCServer, params []interface{}) map[string]interface{} {
	//TODO
	return RpcResultUnsupported
}

// A JSON example for submitblock method as following:
//   {"jsonrpc": "2.0", "method": "submitblock", "params": ["raw block in hex"], "id": 0}
func submitBlock(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 1 {
		return RpcResultNil
	}
	switch params[0].(type) {
	case string:
		str := params[0].(string)
		hex, _ := HexStringToBytes(str)
		var block ledger.Block
		if err := block.Deserialize(bytes.NewReader(hex)); err != nil {
			return RpcResultInvalidBlock
		}
		if err := ledger.DefaultLedger.Blockchain.AddBlock(&block); err != nil {
			return RpcResultInvalidBlock
		}
		if err := s.node.LocalNode().CleanSubmittedTransactions(block.Transactions); err != nil {
			return RpcResultInternalError
		}
		if err := s.node.Xmit(&block); err != nil {
			return RpcResultInternalError
		}
	default:
		return RpcResultInvalidParameter
	}
	return RpcResultSuccess
}

func getNeighbor(s *RPCServer, params []interface{}) map[string]interface{} {
	addr, _ := s.node.GetNeighborAddrs()
	return RpcResult(addr)
}

func getNodeState(s *RPCServer, params []interface{}) map[string]interface{} {
	n := NodeInfo{
		State:    uint(s.node.GetState()),
		Time:     s.node.GetTime(),
		Port:     s.node.GetPort(),
		ID:       s.node.GetID(),
		Version:  s.node.Version(),
		Services: s.node.Services(),
		Relay:    s.node.GetRelay(),
		Height:   s.node.GetHeight(),
		TxnCnt:   s.node.GetTxnCnt(),
		RxTxnCnt: s.node.GetRxTxnCnt(),
	}
	return RpcResult(n)
}

func setDebugInfo(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 1 {
		return RpcResultInvalidParameter
	}
	switch params[0].(type) {
	case float64:
		level := params[0].(float64)
		if err := log.Log.SetDebugLevel(int(level)); err != nil {
			return RpcResultInvalidParameter
		}
	default:
		return RpcResultInvalidParameter
	}
	return RpcResultSuccess
}

func getVersion(s *RPCServer, params []interface{}) map[string]interface{} {
	return RpcResult(config.Version)
}

func getBalance(s *RPCServer, params []interface{}) map[string]interface{} {
	unspent, _ := s.wallet.GetUnspent()
	assets := make(map[Uint256]Fixed64)
	for id, list := range unspent {
		for _, item := range list {
			if _, ok := assets[id]; !ok {
				assets[id] = item.Value
			} else {
				assets[id] += item.Value
			}
		}
	}
	ret := make(map[string]string)
	for id, value := range assets {
		ret[BytesToHexString(id.ToArrayReverse())] = value.String()
	}

	return RpcResult(ret)
}

func registAsset(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 2 {
		return RpcResultNil
	}
	var assetName, assetValue string
	switch params[0].(type) {
	case string:
		assetName = params[0].(string)
	default:
		return RpcResultInvalidParameter
	}
	switch params[1].(type) {
	case string:
		assetValue = params[1].(string)
	default:
		return RpcResultInvalidParameter
	}
	if s.wallet == nil {
		return RpcResult("open wallet first")
	}

	txn, err := MakeRegTransaction(s.wallet, assetName, assetValue)
	if err != nil {
		return RpcResultInternalError
	}

	if errCode := s.VerifyAndSendTx(txn); errCode != ErrNoError {
		return RpcResultInvalidTransaction
	}

	txHash := txn.Hash()
	return RpcResult(BytesToHexString(txHash.ToArrayReverse()))
}

func issueAsset(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 3 {
		return RpcResultNil
	}
	var asset, value, address string
	switch params[0].(type) {
	case string:
		asset = params[0].(string)
	default:
		return RpcResultInvalidParameter
	}
	switch params[1].(type) {
	case string:
		address = params[1].(string)
	default:
		return RpcResultInvalidParameter
	}
	switch params[2].(type) {
	case string:
		value = params[2].(string)
	default:
		return RpcResultInvalidParameter
	}
	if s.wallet == nil {
		return RpcResult("open wallet first")
	}
	tmp, err := HexStringToBytesReverse(asset)
	if err != nil {
		return RpcResult("invalid asset ID")
	}
	var assetID Uint256
	if err := assetID.Deserialize(bytes.NewReader(tmp)); err != nil {
		return RpcResult("invalid asset hash")
	}
	txn, err := MakeIssueTransaction(s.wallet, assetID, address, value)
	if err != nil {
		return RpcResultInternalError
	}

	if errCode := s.VerifyAndSendTx(txn); errCode != ErrNoError {
		return RpcResultInvalidTransaction
	}

	txHash := txn.Hash()
	return RpcResult(BytesToHexString(txHash.ToArrayReverse()))
}

func sendToAddress(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 3 {
		return RpcResultNil
	}
	var asset, address, value string
	switch params[0].(type) {
	case string:
		asset = params[0].(string)
	default:
		return RpcResultInvalidParameter
	}
	switch params[1].(type) {
	case string:
		address = params[1].(string)
	default:
		return RpcResultInvalidParameter
	}
	switch params[2].(type) {
	case string:
		value = params[2].(string)
	default:
		return RpcResultInvalidParameter
	}
	if s.wallet == nil {
		return RpcResult("error : wallet is not opened")
	}

	batchOut := BatchOut{
		Address: address,
		Value:   value,
	}
	tmp, err := HexStringToBytesReverse(asset)
	if err != nil {
		return RpcResult("error: invalid asset ID")
	}
	var assetID Uint256
	if err := assetID.Deserialize(bytes.NewReader(tmp)); err != nil {
		return RpcResult("error: invalid asset hash")
	}
	txn, err := MakeTransferTransaction(s.wallet, assetID, batchOut)
	if err != nil {
		return RpcResult("error: " + err.Error())
	}

	if errCode := s.VerifyAndSendTx(txn); errCode != ErrNoError {
		return RpcResult("error: " + errCode.Error())
	}
	txHash := txn.Hash()
	return RpcResult(BytesToHexString(txHash.ToArrayReverse()))
}

func prepaidAsset(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 3 {
		return RpcResultNil
	}
	var assetName, assetValue, rates string
	switch params[0].(type) {
	case string:
		assetName = params[0].(string)
	default:
		return RpcResultInvalidParameter
	}
	switch params[1].(type) {
	case string:
		assetValue = params[1].(string)
	default:
		return RpcResultInvalidParameter
	}
	switch params[2].(type) {
	case string:
		rates = params[2].(string)
	default:
		return RpcResultInvalidParameter
	}
	if s.wallet == nil {
		return RpcResult("open wallet first")
	}
	tmp, err := HexStringToBytesReverse(assetName)
	if err != nil {
		return RpcResult("error: invalid asset ID")
	}
	var assetID Uint256
	if err := assetID.Deserialize(bytes.NewReader(tmp)); err != nil {
		return RpcResult("error: invalid asset hash")
	}
	txn, err := MakePrepaidTransaction(s.wallet, assetID, assetValue, rates)
	if err != nil {
		return RpcResultInternalError
	}

	if errCode := s.VerifyAndSendTx(txn); errCode != ErrNoError {
		return RpcResultInvalidTransaction
	}

	txHash := txn.Hash()
	return RpcResult(BytesToHexString(txHash.ToArrayReverse()))
}

func withdrawAsset(s *RPCServer, params []interface{}) map[string]interface{} {
	if len(params) < 2 {
		return RpcResultNil
	}

	var assetName, assetValue string
	switch params[0].(type) {
	case string:
		assetName = params[0].(string)
	default:
		return RpcResultInvalidParameter
	}
	switch params[1].(type) {
	case string:
		assetValue = params[1].(string)
	default:
		return RpcResultInvalidParameter
	}
	if s.wallet == nil {
		return RpcResult("open wallet first")
	}

	tmp, err := HexStringToBytesReverse(assetName)
	if err != nil {
		return RpcResult("error: invalid asset ID")
	}
	var assetID Uint256
	if err := assetID.Deserialize(bytes.NewReader(tmp)); err != nil {
		return RpcResult("error: invalid asset hash")
	}

	txn, err := MakeWithdrawTransaction(s.wallet, assetID, assetValue)
	if err != nil {
		return RpcResultInternalError
	}

	if errCode := s.VerifyAndSendTx(txn); errCode != ErrNoError {
		return RpcResultInvalidTransaction
	}

	txHash := txn.Hash()
	return RpcResult(BytesToHexString(txHash.ToArrayReverse()))
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
	encodedPublickKey, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return RpcResultInternalError
	}
	sigChain, err := por.NewSigChain(account, 1, dataHash[:], blockHash[:], encodedPublickKey, encodedPublickKey)
	if err != nil {
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
