package common

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/contract"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/errors"
	netcomm "github.com/nknorg/nkn/net/common"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/por"
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
// params: []
// return: {"result":<result>, "error":<errcode>}
func getLatestBlockHash(s Serverer, params map[string]interface{}) map[string]interface{} {
	hash := ledger.DefaultLedger.Blockchain.CurrentBlockHash()
	ret := map[string]interface{}{
		"height": ledger.DefaultLedger.Blockchain.BlockHeight,
		"hash":   common.BytesToHexString(hash.ToArrayReverse()),
	}
	return respPacking(ret, SUCCESS)
}

// getBlock gets block by height or hash
// params: ["height":<height> | "hash":<hash>]
// return: {"result":<result>, "error":<errcode>}
func getBlock(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	var hash common.Uint256
	if index, ok := params["height"].(float64); ok {
		var err error
		height := uint32(index)
		if hash, err = ledger.DefaultLedger.Store.GetBlockHash(height); err != nil {
			return respPacking(nil, UNKNOWN_HASH)
		}
	} else if str, ok := params["hash"].(string); ok {
		hex, err := common.HexStringToBytesReverse(str)
		if err != nil {
			return respPacking(nil, INVALID_PARAMS)
		}
		if err := hash.Deserialize(bytes.NewReader(hex)); err != nil {
			return respPacking(nil, UNKNOWN_HASH)
		}
	} else {
		return respPacking(nil, INVALID_PARAMS)
	}

	block, err := ledger.DefaultLedger.Store.GetBlock(hash)
	if err != nil {
		return respPacking(nil, UNKNOWN_BLOCK)
	}
	block.Hash()

	var b interface{}
	info, _ := block.MarshalJson()
	json.Unmarshal(info, &b)

	return respPacking(b, SUCCESS)
}

// getBlockCount return The total number of blocks
// params: []
// return: {"result":<result>, "error":<errcode>}
func getBlockCount(s Serverer, params map[string]interface{}) map[string]interface{} {
	return respPacking(ledger.DefaultLedger.Blockchain.BlockHeight+1, SUCCESS)
}

// getChordRingInfo gets the information of Chord
// params: []
// return: {"result":<result>, "error":<errcode>}
func getChordRingInfo(s Serverer, params map[string]interface{}) map[string]interface{} {
	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	return respPacking(node.DumpChordInfo(), SUCCESS)
}

// getLatestBlockHeight gets the latest block height
// params: []
// return: {"result":<result>, "error":<errcode>}
func getLatestBlockHeight(s Serverer, params map[string]interface{}) map[string]interface{} {
	return respPacking(ledger.DefaultLedger.Blockchain.BlockHeight, SUCCESS)
}

//// getBlockHash gets the block hash by height
//// params: [<height>]
//func getBlockHash(s Serverer, params map[string]interface{}) map[string]interface{} {
//	resp := make(map[string]interface{})
//
//	if len(params) < 1 {
//		return nil, INVALID_PARAMS
//	}
//
//	switch params[0].(type) {
//	case float64:
//		height := uint32(params[0].(float64))
//		hash, err := ledger.DefaultLedger.Store.GetBlockHash(height)
//		if err != nil {
//			return nil, UNKNOWN_HASH
//		}
//
//		resp["result"] = common.BytesToHexString(hash.ToArrayReverse())
//		return resp, SUCCESS
//	default:
//		return nil, INVALID_PARAMS
//	}
//}

func GetBlockTransactions(block *ledger.Block) interface{} {
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

// getBlockTxsByHeight gets the transactions of block referenced by height
// params: ["height":<height>]
// return: {"result":<result>, "error":<errcode>}
func getBlockTxsByHeight(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	var err error
	if _, ok := params["height"].(float64); !ok {
		return respPacking(nil, INVALID_PARAMS)
	}
	index := uint32(params["height"].(float64))
	hash, err := ledger.DefaultLedger.Store.GetBlockHash(index)
	if err != nil {
		return respPacking(nil, UNKNOWN_HASH)
	}

	block, err := ledger.DefaultLedger.Store.GetBlock(hash)
	if err != nil {
		return respPacking(nil, UNKNOWN_BLOCK)
	}

	txs := GetBlockTransactions(block)

	return respPacking(txs, SUCCESS)
}

// getConnectionCount gets the the number of Connections
// params: []
// return: {"result":<result>, "error":<errcode>}
func getConnectionCount(s Serverer, params map[string]interface{}) map[string]interface{} {
	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	return respPacking(node.GetConnectionCnt(), SUCCESS)
}

// getRawMemPool gets the transactions in txpool
// params: []
// return: {"result":<result>, "error":<errcode>}
func getRawMemPool(s Serverer, params map[string]interface{}) map[string]interface{} {
	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	txs := []interface{}{}
	txpool := node.GetTxnPool()
	for _, t := range txpool.GetAllTransactions() {
		info, err := t.MarshalJson()
		if err != nil {
			return respPacking(nil, INTERNAL_ERROR)
		}
		var x interface{}
		err = json.Unmarshal(info, &x)
		if err != nil {
			return respPacking(nil, INTERNAL_ERROR)
		}
		txs = append(txs, x)
	}
	//	if len(txs) == 0 {
	//		return respPacking(nil, INTERNAL_ERROR)
	//	}

	return respPacking(txs, SUCCESS)
}

// getTransaction gets the transaction by hash
// params: ["hash":<hash>]
// return: {"result":<result>, "error":<errcode>}
func getTransaction(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	if str, ok := params["hash"].(string); ok {
		hex, err := common.HexStringToBytesReverse(str)
		if err != nil {
			return respPacking(nil, INVALID_PARAMS)
		}
		var hash common.Uint256
		err = hash.Deserialize(bytes.NewReader(hex))
		if err != nil {
			return respPacking(nil, INVALID_PARAMS)
		}
		tx, err := ledger.DefaultLedger.Store.GetTransaction(hash)
		if err != nil {
			return respPacking(nil, UNKNOWN_TRANSACTION)
		}

		tx.Hash()
		var tran interface{}
		info, _ := tx.MarshalJson()
		json.Unmarshal(info, &tran)

		return respPacking(tran, SUCCESS)
	} else {
		return respPacking(nil, INVALID_PARAMS)
	}
}

// sendRawTransaction  sends raw transaction to the block chain
// params: ["tx":<transaction>]
// return: {"result":<result>, "error":<errcode>}
func sendRawTransaction(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	var hash common.Uint256
	if str, ok := params["tx"].(string); ok {
		hex, err := common.HexStringToBytes(str)
		if err != nil {
			return respPacking(nil, INVALID_PARAMS)
		}
		var txn transaction.Transaction
		if err := txn.Deserialize(bytes.NewReader(hex)); err != nil {
			return respPacking(nil, INVALID_TRANSACTION)
		}

		hash = txn.Hash()
		if errCode := VerifyAndSendTx(node, &txn); errCode != errors.ErrNoError {
			return respPacking(nil, INVALID_TRANSACTION)
		}
	} else {
		return respPacking(nil, INVALID_PARAMS)
	}

	return respPacking(common.BytesToHexString(hash.ToArrayReverse()), SUCCESS)
}

// getNeighbor gets neighbors of this node
// params: []
// return: {"result":<result>, "error":<errcode>}
func getNeighbor(s Serverer, params map[string]interface{}) map[string]interface{} {
	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	result, _ := node.GetNeighborAddrs()
	return respPacking(result, SUCCESS)
}

// getNodeState gets the state of this node
// params: []
// return: {"result":<result>, "error":<errcode>}
func getNodeState(s Serverer, params map[string]interface{}) map[string]interface{} {
	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	key, _ := node.GetPubKey().EncodePoint(true)
	n := netcomm.NodeInfo{
		SyncState: protocol.SyncStateString[node.GetSyncState()],
		Time:      node.GetTime(),
		Addr:      node.GetAddrStr(),
		JsonPort:  node.GetHttpJsonPort(),
		WsPort:    node.GetWsPort(),
		ID:        node.GetID(),
		Version:   node.Version(),
		Height:    node.GetHeight(),
		PubKey:    hex.EncodeToString(key),
		TxnCnt:    node.GetTxnCnt(),
		RxTxnCnt:  node.GetRxTxnCnt(),
		ChordID:   hex.EncodeToString(node.GetChordAddr()),
	}

	return respPacking(n, SUCCESS)
}

// setDebugInfo sets log level
// params: ["level":<log leverl>]
// return: {"result":<result>, "error":<errcode>}
func setDebugInfo(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	if level, ok := params["level"].(float64); ok {
		if err := log.Log.SetDebugLevel(int(level)); err != nil {
			return respPacking(nil, INTERNAL_ERROR)
		}
	} else {
		return respPacking(nil, INVALID_PARAMS)
	}

	return respPacking(nil, SUCCESS)
}

// getVersion gets version of this server
// params: []
// return: {"result":<result>, "error":<errcode>}
func getVersion(s Serverer, params map[string]interface{}) map[string]interface{} {
	return respPacking(config.Version, SUCCESS)
}

// getBalance gets the balance of the wallet used by this server
// params: []
// return: {"result":<result>, "error":<errcode>}
func getBalance(s Serverer, params map[string]interface{}) map[string]interface{} {
	wallet, err := s.GetWallet()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	unspent, _ := wallet.GetUnspent()
	assets := make(map[common.Uint256]common.Fixed64)
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
		ret[common.BytesToHexString(id.ToArrayReverse())] = value.String()
	}

	return respPacking(ret, SUCCESS)
}

// registAsset regist an asset to blockchain
// params: ["name":<name>, "value":<value>]
// return: {"result":<result>, "error":<errcode>}
func registAsset(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 2 {
		return respPacking(nil, INVALID_PARAMS)
	}

	assetName, ok1 := params["name"].(string)
	assetValue, ok2 := params["value"].(string)
	if !ok1 || !ok2 {
		return respPacking(nil, INVALID_PARAMS)
	}

	wallet, err := s.GetWallet()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	txn, err := MakeRegTransaction(wallet, assetName, assetValue)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	if errCode := VerifyAndSendTx(node, txn); errCode != errors.ErrNoError {
		return respPacking(nil, INVALID_TRANSACTION)
	}

	txHash := txn.Hash()
	return respPacking(common.BytesToHexString(txHash.ToArrayReverse()), SUCCESS)
}

// issueAsset issue an asset to an address
// params: ["assetid":<assetd>, "address":<address>, "value":<value>]
// return: {"result":<result>, "error":<errcode>}
func issueAsset(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 3 {
		return respPacking(nil, INVALID_PARAMS)
	}

	asset, ok1 := params["assetid"].(string)
	address, ok2 := params["address"].(string)
	value, ok3 := params["value"].(string)
	if !ok1 || !ok2 || !ok3 {
		return respPacking(nil, INVALID_PARAMS)
	}

	wallet, err := s.GetWallet()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	tmp, err := common.HexStringToBytesReverse(asset)
	if err != nil {
		return respPacking(nil, INVALID_PARAMS)
	}
	var assetID common.Uint256
	if err := assetID.Deserialize(bytes.NewReader(tmp)); err != nil {
		return respPacking(nil, INVALID_ASSET)
	}
	txn, err := MakeIssueTransaction(wallet, assetID, address, value)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	if errCode := VerifyAndSendTx(node, txn); errCode != errors.ErrNoError {
		return respPacking(nil, INVALID_TRANSACTION)
	}

	txHash := txn.Hash()
	return respPacking(common.BytesToHexString(txHash.ToArrayReverse()), SUCCESS)
}

// sendToAddress transfers asset to an address
// params: ["assetid":<assetid>, "addresss":<address>, "value":<value>]
// return: {"result":<result>, "error":<errcode>}
func sendToAddress(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 3 {
		return respPacking(nil, INVALID_PARAMS)
	}

	asset, ok1 := params["assetid"].(string)
	address, ok2 := params["address"].(string)
	value, ok3 := params["value"].(string)
	if !ok1 || !ok2 || !ok3 {
		return respPacking(nil, INVALID_PARAMS)
	}

	wallet, err := s.GetWallet()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	batchOut := BatchOut{
		Address: address,
		Value:   value,
	}
	tmp, err := common.HexStringToBytesReverse(asset)
	if err != nil {
		return respPacking(nil, INVALID_PARAMS)
	}
	var assetID common.Uint256
	if err := assetID.Deserialize(bytes.NewReader(tmp)); err != nil {
		return respPacking(nil, INVALID_ASSET)
	}
	txn, err := MakeTransferTransaction(wallet, assetID, batchOut)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	if errCode := VerifyAndSendTx(node, txn); errCode != errors.ErrNoError {
		return respPacking(nil, INVALID_TRANSACTION)
	}

	txHash := txn.Hash()
	return respPacking(common.BytesToHexString(txHash.ToArrayReverse()), SUCCESS)
}

// prepaid prepaid asset to system
// params: ["assetid":<assetid>, "vaule":<value>, "rates":<rates>]
// return: {"result":<result>, "error":<errcode>}
func prepaidAsset(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 3 {
		return respPacking(nil, INVALID_PARAMS)
	}

	asset, ok1 := params["assetid"].(string)
	assetValue, ok2 := params["value"].(string)
	rates, ok3 := params["rates"].(string)
	if !ok1 || !ok2 || !ok3 {
		return respPacking(nil, INVALID_PARAMS)
	}

	wallet, err := s.GetWallet()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	tmp, err := common.HexStringToBytesReverse(asset)
	if err != nil {
		return respPacking(nil, INVALID_PARAMS)
	}
	var assetID common.Uint256
	if err := assetID.Deserialize(bytes.NewReader(tmp)); err != nil {
		return respPacking(nil, INVALID_ASSET)
	}
	txn, err := MakePrepaidTransaction(wallet, assetID, assetValue, rates)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	if errCode := VerifyAndSendTx(node, txn); errCode != errors.ErrNoError {
		return respPacking(nil, INVALID_TRANSACTION)
	}

	txHash := txn.Hash()
	return respPacking(common.BytesToHexString(txHash.ToArrayReverse()), SUCCESS)
}

// registerName register name to address
// params: ["name":<name>]
// return: {"result":<result>, "error":<errcode>}
func registerName(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	name, ok := params["name"].(string)
	if !ok {
		return respPacking(nil, INVALID_PARAMS)
	}

	wallet, err := s.GetWallet()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	txn, err := MakeRegisterNameTransaction(wallet, name)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	if errCode := VerifyAndSendTx(node, txn); errCode != errors.ErrNoError {
		return respPacking(nil, INVALID_TRANSACTION)
	}

	txHash := txn.Hash()
	return respPacking(common.BytesToHexString(txHash.ToArrayReverse()), SUCCESS)
}

// deleteName register name to address
// params: []
// return: {"result":<result>, "error":<errcode>}
func deleteName(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	wallet, err := s.GetWallet()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	txn, err := MakeDeleteNameTransaction(wallet)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	if errCode := VerifyAndSendTx(node, txn); errCode != errors.ErrNoError {
		return respPacking(nil, INVALID_TRANSACTION)
	}

	txHash := txn.Hash()
	return respPacking(common.BytesToHexString(txHash.ToArrayReverse()), SUCCESS)
}

// withdraw withdraw asset from system
// params: ["assetid":<assetid>, "value":<value>]
// return: {"result":<result>, "error":<errcode>}
func withdrawAsset(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 2 {
		return respPacking(nil, INVALID_PARAMS)
	}

	asset, ok1 := params["assetid"].(string)
	assetValue, ok2 := params["value"].(string)
	if !ok1 || !ok2 {
		return respPacking(nil, INVALID_PARAMS)
	}

	wallet, err := s.GetWallet()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	tmp, err := common.HexStringToBytesReverse(asset)
	if err != nil {
		return respPacking(nil, INVALID_PARAMS)
	}
	var assetID common.Uint256
	if err := assetID.Deserialize(bytes.NewReader(tmp)); err != nil {
		return respPacking(nil, INVALID_ASSET)
	}

	txn, err := MakeWithdrawTransaction(wallet, assetID, assetValue)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	if errCode := VerifyAndSendTx(node, txn); errCode != errors.ErrNoError {
		return respPacking(nil, INVALID_TRANSACTION)
	}

	txHash := txn.Hash()
	return respPacking(common.BytesToHexString(txHash.ToArrayReverse()), SUCCESS)
}

// commitPor send por transaction
// params: ["sigchain":<sigchain>]
// return: {"result":<result>, "error":<errcode>}
func commitPor(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	var sigChain []byte
	if str, ok := params["sigchain"].(string); ok {
		var err error
		sigChain, err = common.HexStringToBytes(str)
		if err != nil {
			return respPacking(nil, INVALID_PARAMS)
		}
	} else {
		return respPacking(nil, INVALID_PARAMS)
	}

	wallet, err := s.GetWallet()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	txn, err := MakeCommitTransaction(wallet, sigChain)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	if errCode := VerifyAndSendTx(node, txn); errCode != errors.ErrNoError {
		return respPacking(nil, INVALID_TRANSACTION)
	}

	txHash := txn.Hash()
	return respPacking(common.BytesToHexString(txHash.ToArrayReverse()), SUCCESS)
}

// sigchaintest send por transaction
// params: []
// return: {"result":<result>, "error":<errcode>}
func sigchaintest(s Serverer, params map[string]interface{}) map[string]interface{} {
	wallet, err := s.GetWallet()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	dataHash := common.Uint256{}
	currentHeight := ledger.DefaultLedger.Store.GetHeight()
	blockHash, err := ledger.DefaultLedger.Store.GetBlockHash(currentHeight - 1)
	if err != nil {
		return respPacking(nil, UNKNOWN_HASH)
	}

	node, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	srcID := node.GetChordAddr()
	encodedPublickKey, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	mining := false
	if node.GetSyncState() == protocol.PersistFinished {
		mining = true
	}
	sigChain, err := por.NewSigChain(account, 1, dataHash[:], blockHash[:], srcID,
		encodedPublickKey, encodedPublickKey, mining)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	if err := sigChain.Sign(srcID, encodedPublickKey, mining, account); err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	if err := sigChain.Sign(srcID, encodedPublickKey, mining, account); err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	buf, err := proto.Marshal(sigChain)
	txn, err := MakeCommitTransaction(wallet, buf)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	if errCode := VerifyAndSendTx(node, txn); errCode != errors.ErrNoError {
		return respPacking(nil, INVALID_TRANSACTION)
	}

	txHash := txn.Hash()
	return respPacking(common.BytesToHexString(txHash.ToArrayReverse()), SUCCESS)
}

// getWsAddr get a websocket address
// params: ["address":<address>]
// return: {"result":<result>, "error":<errcode>}
func getWsAddr(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	if str, ok := params["address"].(string); ok {
		clientID, _, err := address.ParseClientAddress(str)
		node, err := s.GetNetNode()
		if err != nil {
			return respPacking(nil, INTERNAL_ERROR)
		}
		addr, err := node.FindWsAddr(clientID)
		if err != nil {
			log.Error("Cannot get websocket address")
			return respPacking(nil, INTERNAL_ERROR)
		}
		return respPacking(addr, SUCCESS)
	} else {
		return respPacking(nil, INTERNAL_ERROR)
	}
}

// getHttpProxyAddr get a websocket address
// params: ["address":<address>]
// return: {"result":<result>, "error":<errcode>}
func getHttpProxyAddr(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	if str, ok := params["address"].(string); ok {
		clientID, _, err := address.ParseClientAddress(str)
		node, err := s.GetNetNode()
		if err != nil {
			return respPacking(nil, INTERNAL_ERROR)
		}
		addr, err := node.FindHttpProxyAddr(clientID)
		if err != nil {
			log.Error("Cannot get http proxy address")
			return respPacking(nil, INTERNAL_ERROR)
		}
		return respPacking(addr, SUCCESS)
	} else {
		return respPacking(nil, INTERNAL_ERROR)
	}
}

// getTotalIssued gets total issued asset
// params: ["assetid":<assetid>]
// return: {"result":<result>, "error":<errcode>}
func getTotalIssued(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	assetid, ok := params["assetid"].(string)
	if !ok {
		return respPacking(nil, INVALID_PARAMS)
	}

	bys, err := common.HexStringToBytesReverse(assetid)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	var assetHash common.Uint256
	if err := assetHash.Deserialize(bytes.NewReader(bys)); err != nil {
		return respPacking(nil, INVALID_ASSET)
	}

	amount, err := ledger.DefaultLedger.Store.GetQuantityIssued(assetHash)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	val := float64(amount) / math.Pow(10, 8)
	return respPacking(val, SUCCESS)
}

// getAssetByHash gets asset by hash
// params: ["hash":<hash>]
// return: {"result":<result>, "error":<errcode>}
func getAssetByHash(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	str, ok := params["hash"].(string)
	if !ok {
		return respPacking(nil, INVALID_PARAMS)
	}

	hex, err := common.HexStringToBytesReverse(str)
	if err != nil {
		return respPacking(nil, INVALID_PARAMS)
	}

	var hash common.Uint256
	err = hash.Deserialize(bytes.NewReader(hex))
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	asset, err := ledger.DefaultLedger.Store.GetAsset(hash)
	if err != nil {
		return respPacking(nil, INVALID_ASSET)
	}

	return respPacking(asset, SUCCESS)
}

// getBalanceByAddr gets balance by address
// params: ["address":<address>]
// return: {"result":<result>, "error":<errcode>}
func getBalanceByAddr(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	addr, ok := params["address"].(string)
	if !ok {
		return respPacking(nil, INVALID_PARAMS)
	}

	var programHash common.Uint160
	programHash, err := common.ToScriptHash(addr)
	if err != nil {
		return respPacking(nil, INVALID_PARAMS)
	}

	unspends, err := ledger.DefaultLedger.Store.GetUnspentsFromProgramHash(programHash)
	var balance common.Fixed64 = 0
	for _, u := range unspends {
		for _, v := range u {
			balance = balance + v.Value
		}
	}

	val := float64(balance) / math.Pow(10, 8)
	return respPacking(val, SUCCESS)
}

// getBalanceByAsset gets balance by asset
// params: ["address":<address>,"assetid":<assetid>]
// return: {"result":<result>, "error":<errcode>}
func getBalanceByAsset(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 2 {
		return respPacking(nil, INVALID_PARAMS)
	}

	addr, ok := params["address"].(string)
	assetid, k := params["assetid"].(string)
	if !ok || !k {
		return respPacking(nil, INVALID_PARAMS)
	}

	var programHash common.Uint160
	programHash, err := common.ToScriptHash(addr)
	if err != nil {
		return respPacking(nil, UNKNOWN_HASH)
	}

	unspends, err := ledger.DefaultLedger.Store.GetUnspentsFromProgramHash(programHash)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	var balance common.Fixed64 = 0
	for k, u := range unspends {
		assid := common.BytesToHexString(k.ToArrayReverse())
		for _, v := range u {
			if assetid == assid {
				balance = balance + v.Value
			}
		}
	}

	val := float64(balance) / math.Pow(10, 8)
	return respPacking(val, SUCCESS)
}

// getUnspendOutput gets unspents by address
// params: ["address":<address>, "assetid":<assetid>]
// return: {"result":<result>, "error":<errcode>}
func getUnspendOutput(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 2 {
		return respPacking(nil, INVALID_PARAMS)
	}

	addr, ok := params["address"].(string)
	assetid, k := params["assetid"].(string)
	if !ok || !k {
		return respPacking(nil, INVALID_PARAMS)

	}

	var programHash common.Uint160
	var assetHash common.Uint256
	programHash, err := common.ToScriptHash(addr)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)

	}

	bys, err := common.HexStringToBytesReverse(assetid)
	if err != nil {
		return respPacking(nil, INVALID_PARAMS)

	}

	if err := assetHash.Deserialize(bytes.NewReader(bys)); err != nil {
		return respPacking(nil, INVALID_ASSET)
	}

	type UTXOUnspentInfo struct {
		Txid  string
		Index uint32
		Value float64
	}

	infos, err := ledger.DefaultLedger.Store.GetUnspentFromProgramHash(programHash, assetHash)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	var UTXOoutputs []UTXOUnspentInfo
	for _, v := range infos {
		val := float64(v.Value) / math.Pow(10, 8)
		UTXOoutputs = append(UTXOoutputs, UTXOUnspentInfo{Txid: common.BytesToHexString(v.Txid.ToArrayReverse()), Index: v.Index, Value: val})
	}

	return respPacking(UTXOoutputs, SUCCESS)
}

// getUnspends gets all assets by address
// params: ["address":<address>]
// return: {"result":<result>, "error":<errcode>}
func getUnspends(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)

	}

	addr, ok := params["address"].(string)
	if !ok {
		return respPacking(nil, INVALID_PARAMS)

	}
	var programHash common.Uint160

	programHash, err := common.ToScriptHash(addr)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
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
		assetid := common.BytesToHexString(k.ToArrayReverse())
		asset, err := ledger.DefaultLedger.Store.GetAsset(k)
		if err != nil {
			return respPacking(nil, INVALID_ASSET)
		}

		var unspendsInfo []UTXOUnspentInfo
		for _, v := range u {
			val := float64(v.Value) / math.Pow(10, 8)
			unspendsInfo = append(unspendsInfo, UTXOUnspentInfo{common.BytesToHexString(v.Txid.ToArrayReverse()), v.Index, val})
		}

		results = append(results, Result{assetid, asset.Name, unspendsInfo})
	}

	return respPacking(results, SUCCESS)
}

func VerifyAndSendTx(n protocol.Noder, txn *transaction.Transaction) errors.ErrCode {
	if errCode := n.AppendTxnPool(txn); errCode != errors.ErrNoError {
		log.Warning("Can NOT add the transaction to TxnPool")
		return errCode
	}
	if err := n.Xmit(txn); err != nil {
		log.Error("Xmit Tx Error:Xmit transaction failed.", err)
		return errors.ErrXmitFail
	}
	return errors.ErrNoError
}

// getAddressByName get address by name
// params: ["name":<name>]
// return: {"result":<result>, "error":<errcode>}
func getAddressByName(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	name, ok := params["name"].(string)
	if !ok {
		return respPacking(nil, INVALID_PARAMS)
	}

	publicKey, err := ledger.DefaultLedger.Store.GetRegistrant(name)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	script, err := contract.CreateSignatureRedeemScriptWithEncodedPublicKey(publicKey)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	scriptHash, err := common.ToCodeHash(script)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	address, err := scriptHash.ToAddress()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	return respPacking(address, SUCCESS)
}

// getSubscribers get subscribers by topic
// params: ["topic":<topic>]
// return: {"result":<result>, "error":<errcode>}
func getSubscribers(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	topic, ok := params["topic"].(string)
	if !ok {
		return respPacking(nil, INVALID_PARAMS)
	}

	subscribers := ledger.DefaultLedger.Store.GetSubscribers(topic)
	return respPacking(subscribers, SUCCESS)
}

// findSuccessorAddrs find the successors of a key
// params: ["address":<address>]
// return: {"result":<result>, "error":<errcode>}
func findSuccessorAddrs(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	if str, ok := params["key"].(string); ok {
		key, err := hex.DecodeString(str)
		if err != nil {
			log.Error("Invalid hex string:", err)
			return respPacking(nil, INVALID_PARAMS)
		}

		node, err := s.GetNetNode()
		if err != nil {
			log.Error("Cannot get node:", err)
			return respPacking(nil, INTERNAL_ERROR)
		}

		addrs, err := node.FindSuccessorAddrs(key, config.MinNumSuccessors)
		if err != nil {
			log.Error("Cannot get successor address:", err)
			return respPacking(nil, INTERNAL_ERROR)
		}

		return respPacking(addrs, SUCCESS)
	} else {
		return respPacking(nil, INTERNAL_ERROR)
	}
}

// Depracated, use findSuccessorAddrs instead
func findSuccessorAddr(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	if str, ok := params["key"].(string); ok {
		key, err := hex.DecodeString(str)
		if err != nil {
			log.Error("Invalid hex string:", err)
			return respPacking(nil, INVALID_PARAMS)
		}

		node, err := s.GetNetNode()
		if err != nil {
			log.Error("Cannot get node:", err)
			return respPacking(nil, INTERNAL_ERROR)
		}

		addrs, err := node.FindSuccessorAddrs(key, 1)
		if err != nil || len(addrs) == 0 {
			log.Error("Cannot get successor address:", err)
			return respPacking(nil, INTERNAL_ERROR)
		}

		return respPacking(addrs[0], SUCCESS)
	} else {
		return respPacking(nil, INTERNAL_ERROR)
	}
}

var InitialAPIHandlers = map[string]APIHandler{
	"getlatestblockhash":   {Handler: getLatestBlockHash, AccessCtrl: BIT_JSONRPC},
	"getblock":             {Handler: getBlock, AccessCtrl: BIT_JSONRPC | BIT_WEBSOCKET},
	"getblockcount":        {Handler: getBlockCount, AccessCtrl: BIT_JSONRPC},
	"getlatestblockheight": {Handler: getLatestBlockHeight, AccessCtrl: BIT_JSONRPC | BIT_WEBSOCKET},
	"getblocktxsbyheight":  {Handler: getBlockTxsByHeight, AccessCtrl: BIT_JSONRPC},
	"getconnectioncount":   {Handler: getConnectionCount, AccessCtrl: BIT_JSONRPC | BIT_WEBSOCKET},
	"getrawmempool":        {Handler: getRawMemPool, AccessCtrl: BIT_JSONRPC},
	"gettransaction":       {Handler: getTransaction, AccessCtrl: BIT_JSONRPC | BIT_WEBSOCKET},
	"sendrawtransaction":   {Handler: sendRawTransaction, AccessCtrl: BIT_JSONRPC | BIT_WEBSOCKET},
	"getwsaddr":            {Handler: getWsAddr, AccessCtrl: BIT_JSONRPC},
	"gethttpproxyaddr":     {Handler: getHttpProxyAddr, AccessCtrl: BIT_JSONRPC},
	"getversion":           {Handler: getVersion, AccessCtrl: BIT_JSONRPC},
	"getneighbor":          {Handler: getNeighbor, AccessCtrl: BIT_JSONRPC},
	"getnodestate":         {Handler: getNodeState, AccessCtrl: BIT_JSONRPC},
	"getchordringinfo":     {Handler: getChordRingInfo, AccessCtrl: BIT_JSONRPC},
	"getunspendoutput":     {Handler: getUnspendOutput, AccessCtrl: BIT_JSONRPC},
	"getbalance":           {Handler: getBalance},
	"setdebuginfo":         {Handler: setDebugInfo},
	"registasset":          {Handler: registAsset},
	"issueasset":           {Handler: issueAsset},
	"sendtoaddress":        {Handler: sendToAddress},
	"prepaidasset":         {Handler: prepaidAsset},
	"registername":         {Handler: registerName},
	"deletename":           {Handler: deleteName},
	"withdrawasset":        {Handler: withdrawAsset},
	"commitpor":            {Handler: commitPor},
	"sigchaintest":         {Handler: sigchaintest},
	"gettotalissued":       {Handler: getTotalIssued},
	"getassetbyhash":       {Handler: getAssetByHash},
	"getbalancebyaddr":     {Handler: getBalanceByAddr, AccessCtrl: BIT_JSONRPC},
	"getbalancebyasset":    {Handler: getBalanceByAsset},
	"getunspends":          {Handler: getUnspends},
	"getaddressbyname":     {Handler: getAddressByName, AccessCtrl: BIT_JSONRPC},
	"getsubscribers":       {Handler: getSubscribers, AccessCtrl: BIT_JSONRPC},
	"findsuccessoraddr":    {Handler: findSuccessorAddr, AccessCtrl: BIT_JSONRPC},
	"findsuccessoraddrs":   {Handler: findSuccessorAddrs, AccessCtrl: BIT_JSONRPC},
}
