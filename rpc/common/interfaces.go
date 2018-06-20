package common

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/net/chord"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
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

func getBlockHash(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 1 {
		return nil, INTERNAL_ERROR
	}

	switch params[0].(type) {
	case float64:
		height := uint32(params[0].(float64))
		hash, err := ledger.DefaultLedger.Store.GetBlockHash(height)
		if err != nil {
			return nil, INTERNAL_ERROR
		}

		resp["result"] = common.BytesToHexString(hash.ToArrayReverse())
		return resp, SUCCESS
	default:
		return nil, INTERNAL_ERROR
	}
}

func getBlockTxsByHeight(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 1 {
		return nil, INTERNAL_ERROR
	}

	var err error
	if _, ok := params[0].(float64); !ok {
		return nil, INTERNAL_ERROR
	}
	index := uint32(params[0].(float64))
	hash, err := ledger.DefaultLedger.Store.GetBlockHash(index)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	block, err := ledger.DefaultLedger.Store.GetBlock(hash)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	txs := func(block *ledger.Block) interface{} {
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
	}(block)

	resp["result"] = txs
	return resp, SUCCESS
}

func getConnectionCount(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	node, err := s.GetNetNode()
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	resp["result"] = node.GetConnectionCnt()

	return resp, SUCCESS
}

func getRawMemPool(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	node, err := s.GetNetNode()
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	txs := []interface{}{}
	txpool := node.GetTxnPool()
	for _, t := range txpool.GetAllTransactions() {
		info, err := t.MarshalJson()
		if err != nil {
			return nil, INTERNAL_ERROR
		}
		var x interface{}
		err = json.Unmarshal(info, &x)
		if err != nil {
			return nil, INTERNAL_ERROR
		}
		txs = append(txs, x)
	}
	if len(txs) == 0 {
		return nil, INTERNAL_ERROR
	}

	resp["result"] = txs

	return resp, SUCCESS
}

func getTransaction(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 1 {
		return nil, INTERNAL_ERROR
	}

	switch params[0].(type) {
	case string:
		str := params[0].(string)
		hex, err := common.HexStringToBytesReverse(str)
		if err != nil {
			return nil, INTERNAL_ERROR
		}
		var hash common.Uint256
		err = hash.Deserialize(bytes.NewReader(hex))
		if err != nil {
			return nil, INTERNAL_ERROR
		}
		tx, err := ledger.DefaultLedger.Store.GetTransaction(hash)
		if err != nil {
			return nil, INTERNAL_ERROR
		}

		tx.Hash()
		var tran interface{}
		info, _ := tx.MarshalJson()
		json.Unmarshal(info, &tran)

		resp["result"] = tran
		return resp, SUCCESS
	default:
		return nil, INTERNAL_ERROR
	}
}

func sendRawTransaction(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 1 {
		return nil, INTERNAL_ERROR
	}

	var hash common.Uint256
	switch params[0].(type) {
	case string:
		str := params[0].(string)
		hex, err := common.HexStringToBytes(str)
		if err != nil {
			return nil, INTERNAL_ERROR
		}
		var txn transaction.Transaction
		if err := txn.Deserialize(bytes.NewReader(hex)); err != nil {
			return nil, INTERNAL_ERROR
		}

		hash = txn.Hash()
		if errCode := s.VerifyAndSendTx(&txn); errCode != errors.ErrNoError {
			return nil, INTERNAL_ERROR
		}
	default:
		return nil, INTERNAL_ERROR
	}

	resp["result"] = common.BytesToHexString(hash.ToArrayReverse())
	return resp, SUCCESS
}

func getNeighbor(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	node, err := s.GetNetNode()
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	resp["result"], _ = node.GetNeighborAddrs()
	return resp, SUCCESS
}

func getNodeState(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	node, err := s.GetNetNode()
	if err != nil {
		return nil, INTERNAL_ERROR
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
		ChordID  string // Chord ID
	}

	n := NodeInfo{
		State:    uint(node.GetState()),
		Time:     node.GetTime(),
		Port:     node.GetPort(),
		ID:       node.GetID(),
		Version:  node.Version(),
		Services: node.Services(),
		Relay:    node.GetRelay(),
		Height:   node.GetHeight(),
		TxnCnt:   node.GetTxnCnt(),
		RxTxnCnt: node.GetRxTxnCnt(),
		ChordID:  hex.EncodeToString(node.GetChordAddr()),
	}

	resp["result"] = n
	return resp, SUCCESS
}

func setDebugInfo(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 1 {
		return nil, INTERNAL_ERROR
	}

	switch params[0].(type) {
	case float64:
		level := params[0].(float64)
		if err := log.Log.SetDebugLevel(int(level)); err != nil {
			return nil, INTERNAL_ERROR
		}
	default:
		return nil, INTERNAL_ERROR
	}

	resp["result"] = ""
	return resp, SUCCESS
}

func getVersion(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	resp["result"] = config.Version
	return resp, SUCCESS
}

func getBalance(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	wallet, err := s.GetWallet()
	if err != nil {
		return nil, INTERNAL_ERROR
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

	resp["result"] = ret
	return resp, SUCCESS
}

func registAsset(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 2 {
		return nil, INTERNAL_ERROR
	}

	var assetName, assetValue string
	switch params[0].(type) {
	case string:
		assetName = params[0].(string)
	default:
		return nil, INTERNAL_ERROR
	}
	switch params[1].(type) {
	case string:
		assetValue = params[1].(string)
	default:
		return nil, INTERNAL_ERROR
	}

	wallet, err := s.GetWallet()
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	txn, err := MakeRegTransaction(wallet, assetName, assetValue)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	if errCode := s.VerifyAndSendTx(txn); errCode != errors.ErrNoError {
		return nil, INTERNAL_ERROR
	}

	txHash := txn.Hash()
	resp["result"] = common.BytesToHexString(txHash.ToArrayReverse())
	return resp, SUCCESS
}

func issueAsset(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 3 {
		return nil, INTERNAL_ERROR
	}
	var asset, value, address string
	switch params[0].(type) {
	case string:
		asset = params[0].(string)
	default:
		return nil, INTERNAL_ERROR
	}
	switch params[1].(type) {
	case string:
		address = params[1].(string)
	default:
		return nil, INTERNAL_ERROR
	}
	switch params[2].(type) {
	case string:
		value = params[2].(string)
	default:
		return nil, INTERNAL_ERROR
	}

	wallet, err := s.GetWallet()
	if err != nil {
		return nil, INTERNAL_ERROR
	}
	tmp, err := common.HexStringToBytesReverse(asset)
	if err != nil {
		return nil, INTERNAL_ERROR
	}
	var assetID common.Uint256
	if err := assetID.Deserialize(bytes.NewReader(tmp)); err != nil {
		return nil, INTERNAL_ERROR
	}
	txn, err := MakeIssueTransaction(wallet, assetID, address, value)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	if errCode := s.VerifyAndSendTx(txn); errCode != errors.ErrNoError {
		return nil, INTERNAL_ERROR
	}

	txHash := txn.Hash()
	resp["result"] = common.BytesToHexString(txHash.ToArrayReverse())
	return resp, SUCCESS
}

func sendToAddress(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 3 {
		return nil, INTERNAL_ERROR
	}
	var asset, address, value string
	switch params[0].(type) {
	case string:
		asset = params[0].(string)
	default:
		return nil, INTERNAL_ERROR
	}
	switch params[1].(type) {
	case string:
		address = params[1].(string)
	default:
		return nil, INTERNAL_ERROR
	}
	switch params[2].(type) {
	case string:
		value = params[2].(string)
	default:
		return nil, INTERNAL_ERROR
	}

	wallet, err := s.GetWallet()
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	batchOut := BatchOut{
		Address: address,
		Value:   value,
	}
	tmp, err := common.HexStringToBytesReverse(asset)
	if err != nil {
		return nil, INTERNAL_ERROR
	}
	var assetID common.Uint256
	if err := assetID.Deserialize(bytes.NewReader(tmp)); err != nil {
		return nil, INTERNAL_ERROR
	}
	txn, err := MakeTransferTransaction(wallet, assetID, batchOut)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	if errCode := s.VerifyAndSendTx(txn); errCode != errors.ErrNoError {
		return nil, INTERNAL_ERROR
	}
	txHash := txn.Hash()
	resp["result"] = common.BytesToHexString(txHash.ToArrayReverse())
	return resp, SUCCESS
}

func prepaidAsset(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 3 {
		return nil, INTERNAL_ERROR
	}
	var assetName, assetValue, rates string
	switch params[0].(type) {
	case string:
		assetName = params[0].(string)
	default:
		return nil, INTERNAL_ERROR
	}
	switch params[1].(type) {
	case string:
		assetValue = params[1].(string)
	default:
		return nil, INTERNAL_ERROR
	}
	switch params[2].(type) {
	case string:
		rates = params[2].(string)
	default:
		return nil, INTERNAL_ERROR
	}

	wallet, err := s.GetWallet()
	if err != nil {
		return nil, INTERNAL_ERROR
	}
	tmp, err := common.HexStringToBytesReverse(assetName)
	if err != nil {
		return nil, INTERNAL_ERROR
	}
	var assetID common.Uint256
	if err := assetID.Deserialize(bytes.NewReader(tmp)); err != nil {
		return nil, INTERNAL_ERROR
	}
	txn, err := MakePrepaidTransaction(wallet, assetID, assetValue, rates)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	if errCode := s.VerifyAndSendTx(txn); errCode != errors.ErrNoError {
		return nil, INTERNAL_ERROR
	}

	txHash := txn.Hash()
	resp["result"] = common.BytesToHexString(txHash.ToArrayReverse())
	return resp, SUCCESS
}

func withdrawAsset(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 2 {
		return nil, INTERNAL_ERROR
	}

	var assetName, assetValue string
	switch params[0].(type) {
	case string:
		assetName = params[0].(string)
	default:
		return nil, INTERNAL_ERROR
	}
	switch params[1].(type) {
	case string:
		assetValue = params[1].(string)
	default:
		return nil, INTERNAL_ERROR
	}

	wallet, err := s.GetWallet()
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	tmp, err := common.HexStringToBytesReverse(assetName)
	if err != nil {
		return nil, INTERNAL_ERROR
	}
	var assetID common.Uint256
	if err := assetID.Deserialize(bytes.NewReader(tmp)); err != nil {
		return nil, INTERNAL_ERROR
	}

	txn, err := MakeWithdrawTransaction(wallet, assetID, assetValue)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	if errCode := s.VerifyAndSendTx(txn); errCode != errors.ErrNoError {
		return nil, INTERNAL_ERROR
	}

	txHash := txn.Hash()
	resp["result"] = common.BytesToHexString(txHash.ToArrayReverse())
	return resp, SUCCESS
}

func commitPor(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 1 {
		return nil, INTERNAL_ERROR
	}

	var sigChain []byte
	var err error
	switch params[0].(type) {
	case string:
		str := params[0].(string)
		sigChain, err = common.HexStringToBytes(str)
		if err != nil {
			return nil, INTERNAL_ERROR
		}
	default:
		return nil, INTERNAL_ERROR
	}

	wallet, err := s.GetWallet()
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	txn, err := MakeCommitTransaction(wallet, sigChain)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	if errCode := s.VerifyAndSendTx(txn); errCode != errors.ErrNoError {
		return nil, INTERNAL_ERROR
	}

	txHash := txn.Hash()
	resp["result"] = common.BytesToHexString(txHash.ToArrayReverse())
	return resp, SUCCESS
}

func sigchaintest(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	wallet, err := s.GetWallet()
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, INTERNAL_ERROR
	}
	dataHash := common.Uint256{}
	currentHeight := ledger.DefaultLedger.Store.GetHeight()
	blockHash, err := ledger.DefaultLedger.Store.GetBlockHash(currentHeight - 1)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	node, err := s.GetNetNode()
	if err != nil {
		return nil, INTERNAL_ERROR
	}
	srcID := node.GetChordAddr()
	encodedPublickKey, err := account.PubKey().EncodePoint(true)
	if err != nil {
		return nil, INTERNAL_ERROR
	}
	sigChain, err := por.NewSigChain(account, 1, dataHash[:], blockHash[:], srcID, encodedPublickKey, encodedPublickKey)
	if err != nil {
		return nil, INTERNAL_ERROR
	}
	if err := sigChain.Sign(encodedPublickKey, account); err != nil {
		return nil, INTERNAL_ERROR
	}
	if err := sigChain.Sign(encodedPublickKey, account); err != nil {
		return nil, INTERNAL_ERROR
	}
	buf, err := proto.Marshal(sigChain)
	txn, err := MakeCommitTransaction(wallet, buf)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	if errCode := s.VerifyAndSendTx(txn); errCode != errors.ErrNoError {
		return nil, INTERNAL_ERROR
	}

	txHash := txn.Hash()
	resp["result"] = common.BytesToHexString(txHash.ToArrayReverse())
	return resp, SUCCESS
}

func getWsAddr(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 1 {
		return nil, INTERNAL_ERROR
	}
	switch params[0].(type) {
	case string:
		clientID, _, err := address.ParseClientAddress(params[0].(string))
		ring := chord.GetRing()
		if ring == nil {
			log.Error("Empty ring")
			return nil, INTERNAL_ERROR
		}
		vnode, err := ring.GetPredecessor(clientID)
		if err != nil {
			log.Error("Cannot get predecessor")
			return nil, INTERNAL_ERROR
		}
		addr, err := vnode.HttpWsAddr()
		if err != nil {
			log.Error("Cannot get websocket address")
			return nil, INTERNAL_ERROR
		}
		resp["result"] = addr
		return resp, SUCCESS
	default:
		return nil, INTERNAL_ERROR
	}
}

func getTotalIssued(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 1 {
		return nil, INTERNAL_ERROR
	}

	assetid, ok := params[0].(string)
	if !ok {
		return nil, INTERNAL_ERROR
	}

	var assetHash common.Uint256
	bys, err := common.HexStringToBytesReverse(assetid)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	if err := assetHash.Deserialize(bytes.NewReader(bys)); err != nil {
		return nil, INTERNAL_ERROR
	}

	amount, err := ledger.DefaultLedger.Store.GetQuantityIssued(assetHash)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	val := float64(amount) / math.Pow(10, 8)
	resp["result"] = val
	return resp, SUCCESS
}

func getAssetByHash(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 1 {
		return nil, INTERNAL_ERROR
	}

	str, ok := params[0].(string)
	if !ok {
		return nil, INTERNAL_ERROR
	}

	hex, err := common.HexStringToBytesReverse(str)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	var hash common.Uint256
	err = hash.Deserialize(bytes.NewReader(hex))
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	asset, err := ledger.DefaultLedger.Store.GetAsset(hash)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	resp["result"] = asset
	return resp, SUCCESS
}

func getBalanceByAddr(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 1 {
		return nil, INTERNAL_ERROR
	}

	addr, ok := params[0].(string)
	if !ok {
		return nil, INTERNAL_ERROR
	}

	var programHash common.Uint160
	programHash, err := common.ToScriptHash(addr)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	unspends, err := ledger.DefaultLedger.Store.GetUnspentsFromProgramHash(programHash)
	var balance common.Fixed64 = 0
	for _, u := range unspends {
		for _, v := range u {
			balance = balance + v.Value
		}
	}

	val := float64(balance) / math.Pow(10, 8)
	resp["result"] = val
	return resp, SUCCESS
}

func getBalanceByAsset(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 2 {
		return nil, INTERNAL_ERROR
	}

	addr, ok := params[0].(string)
	assetid, k := params[1].(string)
	if !ok || !k {
		return nil, INTERNAL_ERROR
	}

	var programHash common.Uint160
	programHash, err := common.ToScriptHash(addr)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	unspends, err := ledger.DefaultLedger.Store.GetUnspentsFromProgramHash(programHash)
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
	resp["result"] = val
	return resp, SUCCESS
}

func getUnspendOutput(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 2 {
		return nil, INTERNAL_ERROR
	}

	addr, ok := params[0].(string)
	assetid, k := params[1].(string)
	if !ok || !k {
		return nil, INTERNAL_ERROR
	}

	var programHash common.Uint160
	var assetHash common.Uint256
	programHash, err := common.ToScriptHash(addr)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	bys, err := common.HexStringToBytesReverse(assetid)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	if err := assetHash.Deserialize(bytes.NewReader(bys)); err != nil {
		return nil, INTERNAL_ERROR
	}

	type UTXOUnspentInfo struct {
		Txid  string
		Index uint32
		Value float64
	}

	infos, err := ledger.DefaultLedger.Store.GetUnspentFromProgramHash(programHash, assetHash)
	if err != nil {
		return nil, INTERNAL_ERROR
	}

	var UTXOoutputs []UTXOUnspentInfo
	for _, v := range infos {
		val := float64(v.Value) / math.Pow(10, 8)
		UTXOoutputs = append(UTXOoutputs, UTXOUnspentInfo{Txid: common.BytesToHexString(v.Txid.ToArrayReverse()), Index: v.Index, Value: val})
	}

	resp["result"] = UTXOoutputs
	return resp, SUCCESS
}

func getUnspends(s Serverer, params []interface{}) (map[string]interface{}, ErrCode) {
	resp := make(map[string]interface{})

	if len(params) < 1 {
		return nil, INTERNAL_ERROR
	}

	addr, ok := params[0].(string)
	if !ok {
		return nil, INTERNAL_ERROR
	}
	var programHash common.Uint160

	programHash, err := common.ToScriptHash(addr)
	if err != nil {
		return nil, INTERNAL_ERROR
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
			return nil, INTERNAL_ERROR
		}

		var unspendsInfo []UTXOUnspentInfo
		for _, v := range u {
			val := float64(v.Value) / math.Pow(10, 8)
			unspendsInfo = append(unspendsInfo, UTXOUnspentInfo{common.BytesToHexString(v.Txid.ToArrayReverse()), v.Index, val})
		}

		results = append(results, Result{assetid, asset.Name, unspendsInfo})
	}

	resp["result"] = results
	return resp, SUCCESS
}

var InitialAPIHandlers = map[string]APIHandler{
	"getlatestblockhash":   {Handler: getLatestBlockHash, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getblock":             {Handler: getBlock, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getblockcount":        {Handler: getBlockCount, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getchordringinfo":     {Handler: getChordRingInfo, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getlatestblockheight": {Handler: getLatestBlockHeight, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getblockhash":         {Handler: getBlockHash, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getblocktxsbyheight":  {Handler: getBlockTxsByHeight, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getconnectioncount":   {Handler: getConnectionCount, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getrawmempool":        {Handler: getRawMemPool, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"gettransaction":       {Handler: getTransaction, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"sendrawtransaction":   {Handler: sendRawTransaction, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getneighbor":          {Handler: getNeighbor, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getnodestate":         {Handler: getNodeState, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"setdebuginfo":         {Handler: setDebugInfo, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getversion":           {Handler: getVersion, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getbalance":           {Handler: getBalance, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"registasset":          {Handler: registAsset, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"issueasset":           {Handler: issueAsset, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"sendtoaddress":        {Handler: sendToAddress, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"prepaidasset":         {Handler: prepaidAsset, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"withdrawasset":        {Handler: withdrawAsset, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},

	"commitpor":         {Handler: commitPor, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"sigchaintest":      {Handler: sigchaintest, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getwsaddr":         {Handler: getWsAddr, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"gettotalissued":    {Handler: getTotalIssued, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getassetbyhash":    {Handler: getAssetByHash, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getbalancebyaddr":  {Handler: getBalanceByAddr, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getbalancebyasset": {Handler: getBalanceByAsset, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getunspendoutput":  {Handler: getUnspendOutput, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
	"getunspends":       {Handler: getUnspends, AccessCtrl: BIT_JSONRPC | BIT_RESTFUL | BIT_WEBSOCKET},
}
