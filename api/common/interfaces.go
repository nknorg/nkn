package common

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net"
	"strings"

	"github.com/gogo/protobuf/proto"
	. "github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/node"
	"github.com/nknorg/nkn/pb"
	. "github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vm/contract"
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
	height := chain.DefaultLedger.Store.GetHeight()
	hash, err := chain.DefaultLedger.Store.GetBlockHash(height)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	ret := map[string]interface{}{
		"height": height,
		"hash":   hash.ToHexString(),
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
		if hash, err = chain.DefaultLedger.Store.GetBlockHash(height); err != nil {
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

	block, err := chain.DefaultLedger.Store.GetBlock(hash)
	if err != nil {
		return respPacking(nil, UNKNOWN_BLOCK)
	}

	var b interface{}
	info, _ := block.GetInfo()
	json.Unmarshal(info, &b)

	return respPacking(b, SUCCESS)
}

// getBlockCount return The total number of blocks
// params: []
// return: {"result":<result>, "error":<errcode>}
func getBlockCount(s Serverer, params map[string]interface{}) map[string]interface{} {
	return respPacking(chain.DefaultLedger.Store.GetHeight()+1, SUCCESS)
}

// getChordRingInfo gets the information of Chord
// params: []
// return: {"result":<result>, "error":<errcode>}
func getChordRingInfo(s Serverer, params map[string]interface{}) map[string]interface{} {
	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	return respPacking(localNode.GetChordInfo(), SUCCESS)
}

// getLatestBlockHeight gets the latest block height
// params: []
// return: {"result":<result>, "error":<errcode>}
func getLatestBlockHeight(s Serverer, params map[string]interface{}) map[string]interface{} {
	return respPacking(chain.DefaultLedger.Store.GetHeight(), SUCCESS)
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
//		hash, err := chain.DefaultLedger.Store.GetBlockHash(height)
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

func GetBlockTransactions(block *Block) interface{} {
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
		Height:       block.Header.UnsignedHeader.Height,
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
	hash, err := chain.DefaultLedger.Store.GetBlockHash(index)
	if err != nil {
		return respPacking(nil, UNKNOWN_HASH)
	}

	block, err := chain.DefaultLedger.Store.GetBlock(hash)
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
	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	return respPacking(localNode.GetConnectionCnt(), SUCCESS)
}

// getRawMemPool gets the transactions in txpool
// params: []
// return: {"result":<result>, "error":<errcode>}
func getRawMemPool(s Serverer, params map[string]interface{}) map[string]interface{} {
	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	txs := []interface{}{}
	txpool := localNode.GetTxnPool()
	for _, t := range txpool.GetAllTransactions() {
		info, err := t.GetInfo()
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
		tx, err := chain.DefaultLedger.Store.GetTransaction(hash)
		if err != nil {
			return respPacking(nil, UNKNOWN_TRANSACTION)
		}

		tx.Hash()
		var tran interface{}
		info, _ := tx.GetInfo()
		json.Unmarshal(info, &tran)

		return respPacking(tran, SUCCESS)
	} else {
		return respPacking(nil, INVALID_PARAMS)
	}
}

// sendRawTransaction  sends raw transaction to the block chain
// params: ["tx":<transaction>]
// return: {"result":<result>, "error":<errcode>, "details":<details>}
func sendRawTransaction(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	var hash common.Uint256
	if str, ok := params["tx"].(string); ok {
		hex, err := common.HexStringToBytes(str)
		if err != nil {
			return respPacking(nil, INVALID_PARAMS)
		}
		var txn Transaction
		if err := txn.Unmarshal(hex); err != nil {
			return respPacking(nil, INVALID_TRANSACTION)
		}

		hash = txn.Hash()
		if errCode := VerifyAndSendTx(localNode, &txn); errCode != ErrNoError {
			return respPackingDetails(nil, INVALID_TRANSACTION, errCode)
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
	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	return respPacking(localNode.GetNeighborInfo(), SUCCESS)
}

// getNodeState gets the state of this node
// params: []
// return: {"result":<result>, "error":<errcode>}
func getNodeState(s Serverer, params map[string]interface{}) map[string]interface{} {
	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	return respPacking(localNode, SUCCESS)
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

	//TODO nonce
	txn, err := MakeCommitTransaction(wallet, sigChain, 0)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	if errCode := VerifyAndSendTx(localNode, txn); errCode != ErrNoError {
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
	currentHeight := chain.DefaultLedger.Store.GetHeight()
	blockHash, err := chain.DefaultLedger.Store.GetBlockHash(currentHeight - 1)
	if err != nil {
		return respPacking(nil, UNKNOWN_HASH)
	}

	localNode, err := s.GetNetNode()
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	srcID := localNode.GetChordID()
	encodedPublickKey, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	mining := false
	if localNode.GetSyncState() == pb.PersistFinished {
		mining = true
	}
	sigChain, err := pb.NewSigChain(account.PubKey(), account.PrivKey(), 1, dataHash[:], blockHash[:], srcID,
		encodedPublickKey, encodedPublickKey, mining)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	if err := sigChain.Sign(srcID, encodedPublickKey, mining, account.PubKey(), account.PrivKey()); err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	if err := sigChain.Sign(srcID, encodedPublickKey, mining, account.PubKey(), account.PrivKey()); err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}
	buf, err := proto.Marshal(sigChain)
	txn, err := MakeCommitTransaction(wallet, buf, 0)
	if err != nil {
		return respPacking(nil, INTERNAL_ERROR)
	}

	if errCode := VerifyAndSendTx(localNode, txn); errCode != ErrNoError {
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
		localNode, err := s.GetNetNode()
		if err != nil {
			return respPacking(nil, INTERNAL_ERROR)
		}
		addr, err := localNode.FindWsAddr(clientID)
		if err != nil {
			log.Errorf("Find websocket address error: %v", err)
			return respPacking(nil, INTERNAL_ERROR)
		}
		return respPacking(addr, SUCCESS)
	} else {
		return respPacking(nil, INTERNAL_ERROR)
	}
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

	pg, _ := common.ToScriptHash(addr)
	value := chain.DefaultLedger.Store.GetBalance(pg)

	ret := map[string]interface{}{
		"amount": value.String(),
	}

	return respPacking(ret, SUCCESS)
}

// getNonceByAddr gets balance by address
// params: ["address":<address>]
// return: {"result":<result>, "error":<errcode>}
func getNonceByAddr(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	addr, ok := params["address"].(string)
	if !ok {
		return respPacking(nil, INVALID_PARAMS)
	}

	pg, _ := common.ToScriptHash(addr)
	value := chain.DefaultLedger.Store.GetNonce(pg)

	ret := map[string]interface{}{
		"nonce": value,
	}

	return respPacking(ret, SUCCESS)
}

func VerifyAndSendTx(localNode *node.LocalNode, txn *Transaction) ErrCode {
	if err := localNode.AppendTxnPool(txn); err != nil {
		log.Warningf("Can NOT add the transaction to TxnPool: %v", err)
		return ErrAppendTxnPool
	}
	if err := localNode.BroadcastTransaction(txn); err != nil {
		log.Errorf("Broadcast Tx Error: %v", err)
		return ErrXmitFail
	}
	return ErrNoError
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

	publicKey, err := chain.DefaultLedger.Store.GetRegistrant(name)
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
// params: ["topic":<topic>, "bucket":<bucket>]
// return: {"result":<result>, "error":<errcode>}
func getSubscribers(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	topic, ok := params["topic"].(string)
	if !ok {
		return respPacking(nil, INVALID_PARAMS)
	}

	bucket, ok := params["bucket"].(float64)
	if !ok {
		return respPacking(nil, INVALID_PARAMS)
	}

	subscribers := chain.DefaultLedger.Store.GetSubscribers(topic, uint32(bucket))
	return respPacking(subscribers, SUCCESS)
}

// getFirstAvailableTopicBucket get free topic bucket
// params: ["topic":<topic>]
// return: {"result":<result>, "error":<errcode>}
func getFirstAvailableTopicBucket(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	topic, ok := params["topic"].(string)
	if !ok {
		return respPacking(nil, INVALID_PARAMS)
	}

	bucket := chain.DefaultLedger.Store.GetFirstAvailableTopicBucket(topic)
	return respPacking(bucket, SUCCESS)
}

// getTopicBucketsCount get topic buckets count
// params: ["topic":<topic>]
// return: {"result":<result>, "error":<errcode>}
func getTopicBucketsCount(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	topic, ok := params["topic"].(string)
	if !ok {
		return respPacking(nil, INVALID_PARAMS)
	}

	count := chain.DefaultLedger.Store.GetTopicBucketsCount(topic)
	return respPacking(count, SUCCESS)
}

// getMyExtIP get RPC client's external IP
// params: ["address":<address>]
// return: {"result":<result>, "error":<errcode>}
func getMyExtIP(s Serverer, params map[string]interface{}) map[string]interface{} {
	if len(params) < 1 {
		return respPacking(nil, INVALID_PARAMS)
	}

	addr, ok := params["RemoteAddr"].(string)
	if !ok || len(addr) == 0 {
		log.Errorf("Invalid params: [%v, %v]", ok, addr)
		return respPacking(nil, INVALID_PARAMS)
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.LastIndexByte(addr, ':') >= 0 {
			log.Errorf("getMyExtIP met invalid params %v: %v", addr, err)
			return respPacking(nil, INVALID_PARAMS)
		}
		host = addr // addr just only host, without port
	}
	ret := map[string]interface{}{"RemoteAddr": host}
	return respPacking(ret, SUCCESS)
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

		localNode, err := s.GetNetNode()
		if err != nil {
			log.Error("Cannot get node:", err)
			return respPacking(nil, INTERNAL_ERROR)
		}

		addrs, err := localNode.FindSuccessorAddrs(key, config.MinNumSuccessors)
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

		localNode, err := s.GetNetNode()
		if err != nil {
			log.Error("Cannot get node:", err)
			return respPacking(nil, INTERNAL_ERROR)
		}

		addrs, err := localNode.FindSuccessorAddrs(key, 1)
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
	"getversion":           {Handler: getVersion, AccessCtrl: BIT_JSONRPC},
	"getneighbor":          {Handler: getNeighbor, AccessCtrl: BIT_JSONRPC},
	"getnodestate":         {Handler: getNodeState, AccessCtrl: BIT_JSONRPC},
	"getchordringinfo":     {Handler: getChordRingInfo, AccessCtrl: BIT_JSONRPC},
	"setdebuginfo":         {Handler: setDebugInfo},
	"commitpor":            {Handler: commitPor},
	"sigchaintest":         {Handler: sigchaintest},
	"getbalancebyaddr":     {Handler: getBalanceByAddr, AccessCtrl: BIT_JSONRPC},
	"getnoncebyaddr":       {Handler: getNonceByAddr, AccessCtrl: BIT_JSONRPC},

	"getaddressbyname":             {Handler: getAddressByName, AccessCtrl: BIT_JSONRPC},
	"getsubscribers":               {Handler: getSubscribers, AccessCtrl: BIT_JSONRPC},
	"getfirstavailabletopicbucket": {Handler: getFirstAvailableTopicBucket, AccessCtrl: BIT_JSONRPC},
	"gettopicbucketscount":         {Handler: getTopicBucketsCount, AccessCtrl: BIT_JSONRPC},
	"getmyextip":                   {Handler: getMyExtIP, AccessCtrl: BIT_JSONRPC},
	"findsuccessoraddr":            {Handler: findSuccessorAddr, AccessCtrl: BIT_JSONRPC},
	"findsuccessoraddrs":           {Handler: findSuccessorAddrs, AccessCtrl: BIT_JSONRPC},
}
