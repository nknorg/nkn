package common

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net"
	"strings"

	"github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/node"
	"github.com/nknorg/nkn/program"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

const (
	BIT_JSONRPC   byte = 1
	BIT_WEBSOCKET byte = 2
)

type Handler func(Serverer, map[string]interface{}) map[string]interface{}

type APIHandler struct {
	Handler    Handler
	AccessCtrl byte
}

// IsAccessableByJsonrpc return true if the handler is
// able to be invoked by jsonrpc
func (ah *APIHandler) IsAccessableByJsonrpc() bool {
	if ah.AccessCtrl&BIT_JSONRPC != BIT_JSONRPC {
		return false
	}

	return true
}

// IsAccessableByWebsocket return true if the handler is
// able to be invoked by websocket
func (ah *APIHandler) IsAccessableByWebsocket() bool {
	if ah.AccessCtrl&BIT_WEBSOCKET != BIT_WEBSOCKET {
		return false
	}

	return true
}

// getLatestBlockHash gets the latest block hash
// params: {}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getLatestBlockHash(s Serverer, params map[string]interface{}) map[string]interface{} {
	height := chain.DefaultLedger.Store.GetHeight()
	hash, err := chain.DefaultLedger.Store.GetBlockHash(height)
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}
	ret := map[string]interface{}{
		"height": height,
		"hash":   hash.ToHexString(),
	}
	return respPacking(SUCCESS, ret)
}

// getBlock gets block by height or hash
// params: {"height":<height> | "hash":<hash>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getBlock(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	var hash common.Uint256
	if index, ok := params["height"].(float64); ok {
		var err error
		height := uint32(index)
		if hash, err = chain.DefaultLedger.Store.GetBlockHash(height); err != nil {
			return respPacking(UNKNOWN_HASH, err.Error())
		}
	} else if str, ok := params["hash"].(string); ok {
		hex, err := common.HexStringToBytes(str)
		if err != nil {
			return respPacking(INVALID_PARAMS, err.Error())
		}
		if err := hash.Deserialize(bytes.NewReader(hex)); err != nil {
			return respPacking(UNKNOWN_HASH, err.Error())
		}
	} else {
		return respPacking(INVALID_PARAMS, "parameter should be height or hash")
	}

	block, err := chain.DefaultLedger.Store.GetBlock(hash)
	if err != nil {
		return respPacking(UNKNOWN_BLOCK, err.Error())
	}

	var b interface{}
	info, err := block.GetInfo()
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	json.Unmarshal(info, &b)

	return respPacking(SUCCESS, b)
}

// getBlockCount return The total number of blocks
// params: {}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getBlockCount(s Serverer, params map[string]interface{}) map[string]interface{} {
	return respPacking(SUCCESS, chain.DefaultLedger.Store.GetHeight()+1)
}

// getChordRingInfo gets the information of Chord
// params: {}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getChordRingInfo(s Serverer, params map[string]interface{}) map[string]interface{} {
	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}
	return respPacking(SUCCESS, localNode.GetChordInfo())
}

// getLatestBlockHeight gets the latest block height
// params: {}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getLatestBlockHeight(s Serverer, params map[string]interface{}) map[string]interface{} {
	return respPacking(SUCCESS, chain.DefaultLedger.Store.GetHeight())
}

func GetBlockTransactions(block *block.Block) interface{} {
	trans := make([]string, len(block.Transactions))
	for i := 0; i < len(block.Transactions); i++ {
		h := block.Transactions[i].Hash()
		trans[i] = common.BytesToHexString(h.ToArray())
	}
	hash := block.Hash()
	type BlockTransactions struct {
		Hash         string
		Height       uint32
		Transactions []string
	}
	b := BlockTransactions{
		Hash:         common.BytesToHexString(hash.ToArray()),
		Height:       block.Header.UnsignedHeader.Height,
		Transactions: trans,
	}
	return b
}

// getBlockTxsByHeight gets the transactions of block referenced by height
// params: {"height":<height>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getBlockTxsByHeight(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	var err error
	if _, ok := params["height"].(float64); !ok {
		return respPacking(INVALID_PARAMS, "height should be float64")
	}
	index := uint32(params["height"].(float64))
	hash, err := chain.DefaultLedger.Store.GetBlockHash(index)
	if err != nil {
		return respPacking(UNKNOWN_HASH, err.Error())
	}

	block, err := chain.DefaultLedger.Store.GetBlock(hash)
	if err != nil {
		return respPacking(UNKNOWN_BLOCK, err.Error())
	}

	txs := GetBlockTransactions(block)

	return respPacking(SUCCESS, txs)
}

// getConnectionCount gets the the number of Connections
// params: {}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getConnectionCount(s Serverer, params map[string]interface{}) map[string]interface{} {
	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	return respPacking(SUCCESS, localNode.GetConnectionCnt())
}

// getRawMemPool gets the transactions in txpool
// params: {}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getRawMemPool(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	action, ok := params["action"].(string)
	if !ok {
		return respPacking(INVALID_PARAMS, "action should be a string")
	}

	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	txpool := localNode.GetTxnPool()

	switch action {
	case "addresslist":
		programHashes := txpool.GetAddressList()
		addresses := []interface{}{}
		for programHash, count := range programHashes {
			addr, err := programHash.ToAddress()
			if err != nil {
				return respPacking(INTERNAL_ERROR, err.Error())
			}

			info := map[string]interface{}{
				"address": addr,
				"txcount": count,
			}
			addresses = append(addresses, info)
		}

		return respPacking(SUCCESS, addresses)
	case "txnlist":
		addr, ok := params["address"].(string)
		if !ok {
			return respPacking(INVALID_PARAMS, "address should be a string")
		}

		programHash, err := common.ToScriptHash(addr)
		if err != nil {
			return respPacking(INVALID_PARAMS, err.Error())
		}

		txs := []interface{}{}
		for _, txn := range txpool.GetAllTransactionsBySender(programHash) {
			info, err := txn.GetInfo()
			if err != nil {
				return respPacking(INTERNAL_ERROR, err.Error())
			}
			var x interface{}
			err = json.Unmarshal(info, &x)
			if err != nil {
				return respPacking(INTERNAL_ERROR, err.Error())
			}
			txs = append(txs, x)
		}

		return respPacking(SUCCESS, txs)
	default:
		return respPacking(INVALID_PARAMS, "action should be addresslist or txnlist")
	}

}

// getTransaction gets the transaction by hash
// params: {"hash":<hash>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getTransaction(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	str, ok := params["hash"].(string)
	if !ok {
		return respPacking(INVALID_PARAMS, "hash should be a string")
	}

	hex, err := common.HexStringToBytes(str)
	if err != nil {
		return respPacking(INVALID_PARAMS, err.Error())
	}
	var hash common.Uint256
	err = hash.Deserialize(bytes.NewReader(hex))
	if err != nil {
		return respPacking(INVALID_PARAMS, err.Error())
	}
	tx, err := chain.DefaultLedger.Store.GetTransaction(hash)
	if err != nil {
		return respPacking(UNKNOWN_TRANSACTION, err.Error())
	}

	tx.Hash()
	var tran interface{}
	info, err := tx.GetInfo()
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	json.Unmarshal(info, &tran)

	return respPacking(SUCCESS, tran)
}

// sendRawTransaction  sends raw transaction to the block chain
// params: {"tx":<transaction>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func sendRawTransaction(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	var hash common.Uint256
	if str, ok := params["tx"].(string); ok {
		hex, err := common.HexStringToBytes(str)
		if err != nil {
			return respPacking(INVALID_PARAMS, err.Error())
		}
		var txn transaction.Transaction
		if err := txn.Unmarshal(hex); err != nil {
			return respPacking(INVALID_TRANSACTION, err.Error())
		}

		hash = txn.Hash()
		if errCode, err := VerifyAndSendTx(localNode, &txn); errCode != ErrNoError {
			return respPacking(errCode, err.Error())
		}
	} else {
		return respPacking(INVALID_PARAMS, err.Error())
	}

	return respPacking(SUCCESS, common.BytesToHexString(hash.ToArray()))
}

// getNeighbor gets neighbors of this node
// params: {}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getNeighbor(s Serverer, params map[string]interface{}) map[string]interface{} {
	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	return respPacking(SUCCESS, localNode.GetNeighborInfo())
}

// getNodeState gets the state of this node
// params: {}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getNodeState(s Serverer, params map[string]interface{}) map[string]interface{} {
	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	return respPacking(SUCCESS, localNode)
}

// setDebugInfo sets log level
// params: {"level":<log leverl>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func setDebugInfo(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	level, ok := params["level"].(float64)
	if !ok {
		return respPacking(INVALID_PARAMS, "level should be float64")
	}
	if err := log.Log.SetDebugLevel(int(level)); err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	return respPacking(SUCCESS, nil)
}

// getVersion gets version of this server
// params: {}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getVersion(s Serverer, params map[string]interface{}) map[string]interface{} {
	return respPacking(SUCCESS, config.Version)
}

func NodeInfo(addr string, pubkey, id []byte) map[string]string {
	nodeInfo := make(map[string]string)
	nodeInfo["addr"] = addr
	nodeInfo["pubkey"] = common.BytesToHexString(pubkey)
	nodeInfo["id"] = common.BytesToHexString(id)
	return nodeInfo
}

// getWsAddr get a websocket address
// params: {"address":<address>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getWsAddr(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	str, ok := params["address"].(string)
	if !ok {
		return respPacking(INTERNAL_ERROR, "address should be a string")
	}

	clientID, _, _, err := address.ParseClientAddress(str)
	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	addr, pubkey, id, err := localNode.FindWsAddr(clientID)
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	return respPacking(SUCCESS, NodeInfo(addr, pubkey, id))
}

// getBalanceByAddr gets balance by address
// params: {"address":<address>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getBalanceByAddr(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	addr, ok := params["address"].(string)
	if !ok {
		return respPacking(INVALID_PARAMS, "address should be a string")
	}

	pg, err := common.ToScriptHash(addr)
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	value := chain.DefaultLedger.Store.GetBalance(pg)

	ret := map[string]interface{}{
		"amount": value.String(),
	}

	return respPacking(SUCCESS, ret)
}

// getBalanceByAssetID gets balance by address
// params: {"address":<address>, "assetid":<assetid>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func GetBalanceByAssetID(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 2 {
		return respPacking(INVALID_PARAMS, "length of params is less than 2")
	}

	addr, ok := params["address"].(string)
	if !ok {
		return respPacking(INVALID_PARAMS, "address should be a string")
	}

	id, ok := params["assetid"].(string)
	if !ok {
		return respPacking(INVALID_PARAMS, "asset id should be a string")
	}

	hexAssetID, err := common.HexStringToBytes(id)
	if err != nil {
		return respPacking(INVALID_PARAMS, err.Error())
	}

	assetID, err := common.Uint256ParseFromBytes(hexAssetID)
	if err != nil {
		return respPacking(INVALID_PARAMS, err.Error())
	}

	pg, err := common.ToScriptHash(addr)
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	value := chain.DefaultLedger.Store.GetBalanceByAssetID(pg, assetID)
	_, symbol, _, _, err := chain.DefaultLedger.Store.GetAsset(assetID)
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	ret := map[string]interface{}{
		"assetID": id,
		"symbol":  symbol,
		"amount":  value.String(),
	}

	return respPacking(SUCCESS, ret)
}

// getNonceByAddr gets balance by address
// params: {"address":<address>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getNonceByAddr(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	addr, ok := params["address"].(string)
	if !ok {
		return respPacking(INVALID_PARAMS, err.Error())
	}

	pg, err := common.ToScriptHash(addr)
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	persistNonce := chain.DefaultLedger.Store.GetNonce(pg)

	txpool := localNode.GetTxnPool()
	txPoolNonce, err := txpool.GetNonceByTxnPool(pg)
	if err != nil {
		txPoolNonce = persistNonce
	}

	ret := map[string]interface{}{
		"nonce":         persistNonce,
		"nonceInTxPool": txPoolNonce,
		"currentHeight": chain.DefaultLedger.Store.GetHeight(),
	}

	return respPacking(SUCCESS, ret)
}

// getId gets id by publick key
// params: {"publickey":<publickey>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getId(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	publicKey, ok := params["publickey"].(string)
	if !ok {
		return respPacking(INVALID_PARAMS, "publicKey should be")
	}

	pkSlice, err := hex.DecodeString(publicKey)
	if err != nil {
		return respPacking(INVALID_PARAMS, err.Error())
	}

	id, err := chain.DefaultLedger.Store.GetID(pkSlice)
	if err != nil {
		return respPacking(INVALID_PARAMS, err.Error())
	}

	if len(id) == 0 {
		return respPacking(ErrNullID, nil)
	}

	if bytes.Equal(id, crypto.Sha256ZeroHash) {
		return respPacking(ErrZeroID, nil)
	}

	ret := map[string]interface{}{
		"id": common.BytesToHexString(id),
	}

	return respPacking(SUCCESS, ret)
}

func VerifyAndSendTx(localNode *node.LocalNode, txn *transaction.Transaction) (ErrCode, error) {
	if err := localNode.AppendTxnPool(txn); err != nil {
		log.Warningf("Can NOT add the transaction to TxnPool: %v", err)

		if err == chain.ErrIDRegistered || err == chain.ErrDuplicateGenerateIDTxn {
			return ErrDuplicatedTx, err
		}

		return ErrAppendTxnPool, err
	}
	if err := localNode.BroadcastTransaction(txn); err != nil {
		log.Errorf("Broadcast Tx Error: %v", err)
		return ErrXmitFail, err
	}

	return ErrNoError, nil
}

// getAddressByName get address by name
// params: {"name":<name>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getAddressByName(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	name, ok := params["name"].(string)
	if !ok {
		return respPacking(INVALID_PARAMS, "name should be a string")
	}

	publicKey, err := chain.DefaultLedger.Store.GetRegistrant(name)
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}
	if publicKey == nil {
		return respPacking(INTERNAL_ERROR, "no such name registered")
	}

	pubKey, err := crypto.NewPubKeyFromBytes(publicKey)
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	programHash, err := program.CreateProgramHash(pubKey)
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	address, err := programHash.ToAddress()
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	return respPacking(SUCCESS, address)
}

// getSubscribers get subscribers by topic
// params: {"topic":<topic>, "bucket":<bucket>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getSubscribers(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 2 {
		return respPacking(INVALID_PARAMS, "length of params is less than 2")
	}

	topic, ok := params["topic"].(string)
	if !ok {
		return respPacking(INVALID_PARAMS, "topic should be a string")
	}

	bucket, ok := params["bucket"].(float64)
	if !ok {
		return respPacking(INVALID_PARAMS, "bucket should be a string")
	}

	subscribers, err := chain.DefaultLedger.Store.GetSubscribers(topic, uint32(bucket))
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}
	return respPacking(SUCCESS, subscribers)
}

// getAsset get subscribers by topic
// params: {"assetid":<id>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getAsset(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	str, ok := params["assetid"].(string)
	if !ok {
		return respPacking(INVALID_PARAMS, "asset ID should be a string")
	}

	hexAssetID, err := common.HexStringToBytes(str)
	if err != nil {
		return respPacking(INVALID_PARAMS, err.Error())
	}

	assetID, err := common.Uint256ParseFromBytes(hexAssetID)
	if err != nil {
		return respPacking(INVALID_PARAMS, err.Error())
	}

	name, symbol, totalSupply, precision, err := chain.DefaultLedger.Store.GetAsset(assetID)
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	ret := map[string]interface{}{
		"name":        name,
		"symbol":      symbol,
		"totalSupply": totalSupply.String(),
		"precision":   precision,
	}

	return respPacking(SUCCESS, ret)
}

// getFirstAvailableTopicBucket get free topic bucket
// params: {"topic":<topic>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getFirstAvailableTopicBucket(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	topic, ok := params["topic"].(string)
	if !ok {
		return respPacking(INVALID_PARAMS, "topic should be a string")
	}

	bucket := chain.DefaultLedger.Store.GetFirstAvailableTopicBucket(topic)
	return respPacking(SUCCESS, bucket)
}

// getTopicBucketsCount get topic buckets count
// params: {"topic":<topic>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getTopicBucketsCount(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	topic, ok := params["topic"].(string)
	if !ok {
		return respPacking(INVALID_PARAMS, "topic should be a string")
	}

	count, err := chain.DefaultLedger.Store.GetTopicBucketsCount(topic)
	if err != nil {
		return respPacking(INTERNAL_ERROR, err.Error())
	}
	return respPacking(SUCCESS, count)
}

// getMyExtIP get RPC client's external IP
// params: {"address":<address>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func getMyExtIP(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	addr, ok := params["RemoteAddr"].(string)
	if !ok || len(addr) == 0 {
		log.Errorf("Invalid params: [%v, %v]", ok, addr)
		return respPacking(INVALID_PARAMS, "RemoteAddr should be a string")
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.LastIndexByte(addr, ':') >= 0 {
			log.Errorf("getMyExtIP met invalid params %v: %v", addr, err)
			return respPacking(INVALID_PARAMS, err.Error())
		}
		host = addr // addr just only host, without port
	}
	ret := map[string]interface{}{"RemoteAddr": host}

	return respPacking(SUCCESS, ret)
}

// findSuccessorAddrs find the successors of a key
// params: {"address":<address>}
// return: {"resultOrData":<result>|<error data>, "error":<errcode>}
func findSuccessorAddrs(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	str, ok := params["key"].(string)
	if !ok {
		return respPacking(INTERNAL_ERROR, "key should be a string")
	}

	key, err := hex.DecodeString(str)
	if err != nil {
		log.Error("Invalid hex string:", err)
		return respPacking(INVALID_PARAMS, err.Error())
	}

	localNode, err := s.GetNetNode()
	if err != nil {
		log.Error("Cannot get node:", err)
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	addrs, err := localNode.FindSuccessorAddrs(key, config.MinNumSuccessors)
	if err != nil {
		log.Error("Cannot get successor address:", err)
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	return respPacking(SUCCESS, addrs)
}

// Depracated, use findSuccessorAddrs instead
func findSuccessorAddr(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(INVALID_PARAMS, "length of params is less than 1")
	}

	str, ok := params["key"].(string)
	if !ok {
		return respPacking(INTERNAL_ERROR, "key should be a string")
	}

	key, err := hex.DecodeString(str)
	if err != nil {
		log.Error("Invalid hex string:", err)
		return respPacking(INVALID_PARAMS, err.Error())
	}

	localNode, err := s.GetNetNode()
	if err != nil {
		log.Error("Cannot get node:", err)
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	addrs, err := localNode.FindSuccessorAddrs(key, 1)
	if err != nil || len(addrs) == 0 {
		log.Error("Cannot get successor address:", err)
		return respPacking(INTERNAL_ERROR, err.Error())
	}

	return respPacking(SUCCESS, addrs[0])
}

var InitialAPIHandlers = map[string]APIHandler{
	"getlatestblockhash":           {Handler: getLatestBlockHash, AccessCtrl: BIT_JSONRPC},
	"getblock":                     {Handler: getBlock, AccessCtrl: BIT_JSONRPC | BIT_WEBSOCKET},
	"getblockcount":                {Handler: getBlockCount, AccessCtrl: BIT_JSONRPC},
	"getlatestblockheight":         {Handler: getLatestBlockHeight, AccessCtrl: BIT_JSONRPC | BIT_WEBSOCKET},
	"getblocktxsbyheight":          {Handler: getBlockTxsByHeight, AccessCtrl: BIT_JSONRPC},
	"getconnectioncount":           {Handler: getConnectionCount, AccessCtrl: BIT_JSONRPC | BIT_WEBSOCKET},
	"getrawmempool":                {Handler: getRawMemPool, AccessCtrl: BIT_JSONRPC},
	"gettransaction":               {Handler: getTransaction, AccessCtrl: BIT_JSONRPC | BIT_WEBSOCKET},
	"sendrawtransaction":           {Handler: sendRawTransaction, AccessCtrl: BIT_JSONRPC | BIT_WEBSOCKET},
	"getwsaddr":                    {Handler: getWsAddr, AccessCtrl: BIT_JSONRPC},
	"getversion":                   {Handler: getVersion, AccessCtrl: BIT_JSONRPC},
	"getneighbor":                  {Handler: getNeighbor, AccessCtrl: BIT_JSONRPC},
	"getnodestate":                 {Handler: getNodeState, AccessCtrl: BIT_JSONRPC},
	"getchordringinfo":             {Handler: getChordRingInfo, AccessCtrl: BIT_JSONRPC},
	"setdebuginfo":                 {Handler: setDebugInfo},
	"getbalancebyaddr":             {Handler: getBalanceByAddr, AccessCtrl: BIT_JSONRPC},
	"getbalancebyassetid":          {Handler: GetBalanceByAssetID, AccessCtrl: BIT_JSONRPC},
	"getnoncebyaddr":               {Handler: getNonceByAddr, AccessCtrl: BIT_JSONRPC},
	"getid":                        {Handler: getId, AccessCtrl: BIT_JSONRPC},
	"getaddressbyname":             {Handler: getAddressByName, AccessCtrl: BIT_JSONRPC},
	"getsubscribers":               {Handler: getSubscribers, AccessCtrl: BIT_JSONRPC},
	"getasset":                     {Handler: getAsset, AccessCtrl: BIT_JSONRPC},
	"getfirstavailabletopicbucket": {Handler: getFirstAvailableTopicBucket, AccessCtrl: BIT_JSONRPC},
	"gettopicbucketscount":         {Handler: getTopicBucketsCount, AccessCtrl: BIT_JSONRPC},
	"getmyextip":                   {Handler: getMyExtIP, AccessCtrl: BIT_JSONRPC},
	"findsuccessoraddr":            {Handler: findSuccessorAddr, AccessCtrl: BIT_JSONRPC},
	"findsuccessoraddrs":           {Handler: findSuccessorAddrs, AccessCtrl: BIT_JSONRPC},
}
