package httpjson

import (
	"bytes"
	. "nkn-core/common"
	"nkn-core/common/config"
	"nkn-core/common/log"
	"nkn-core/core/ledger"
	tx "nkn-core/core/transaction"
	. "nkn-core/errors"
	"nkn-core/helper"
	"nkn-core/wallet"
)

var Wallet wallet.Wallet

func TransArryByteToHexString(ptx *tx.Transaction) *Transactions {

	trans := new(Transactions)
	trans.TxType = ptx.TxType
	trans.PayloadVersion = ptx.PayloadVersion
	trans.Payload = TransPayloadToHex(ptx.Payload)

	n := 0
	trans.Attributes = make([]TxAttributeInfo, len(ptx.Attributes))
	for _, v := range ptx.Attributes {
		trans.Attributes[n].Usage = v.Usage
		trans.Attributes[n].Data = BytesToHexString(v.Data)
		n++
	}

	n = 0
	trans.UTXOInputs = make([]UTXOTxInputInfo, len(ptx.UTXOInputs))
	for _, v := range ptx.UTXOInputs {
		trans.UTXOInputs[n].ReferTxID = BytesToHexString(v.ReferTxID.ToArrayReverse())
		trans.UTXOInputs[n].ReferTxOutputIndex = v.ReferTxOutputIndex
		n++
	}

	n = 0
	trans.Outputs = make([]TxoutputInfo, len(ptx.Outputs))
	for _, v := range ptx.Outputs {
		trans.Outputs[n].AssetID = BytesToHexString(v.AssetID.ToArrayReverse())
		trans.Outputs[n].Value = v.Value.String()
		address, _ := v.ProgramHash.ToAddress()
		trans.Outputs[n].Address = address
		n++
	}

	n = 0
	trans.Programs = make([]ProgramInfo, len(ptx.Programs))
	for _, v := range ptx.Programs {
		trans.Programs[n].Code = BytesToHexString(v.Code)
		trans.Programs[n].Parameter = BytesToHexString(v.Parameter)
		n++
	}

	mHash := ptx.Hash()
	trans.Hash = BytesToHexString(mHash.ToArrayReverse())

	return trans
}

func getBestBlockHash(params []interface{}) map[string]interface{} {
	hash := ledger.DefaultLedger.Blockchain.CurrentBlockHash()
	return RpcResult(BytesToHexString(hash.ToArrayReverse()))
}

// Input JSON string examples for getblock method as following:
//   {"jsonrpc": "2.0", "method": "getblock", "params": [1], "id": 0}
//   {"jsonrpc": "2.0", "method": "getblock", "params": ["aabbcc.."], "id": 0}
func getBlock(params []interface{}) map[string]interface{} {
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

	blockHead := &BlockHead{
		Version:          block.Header.Version,
		PrevBlockHash:    BytesToHexString(block.Header.PrevBlockHash.ToArrayReverse()),
		TransactionsRoot: BytesToHexString(block.Header.TransactionsRoot.ToArrayReverse()),
		Timestamp:        block.Header.Timestamp,
		Height:           block.Header.Height,
		ConsensusData:    block.Header.ConsensusData,
		NextBookKeeper:   BytesToHexString(block.Header.NextBookKeeper.ToArrayReverse()),
		Program: ProgramInfo{
			Code:      BytesToHexString(block.Header.Program.Code),
			Parameter: BytesToHexString(block.Header.Program.Parameter),
		},
		Hash: BytesToHexString(hash.ToArrayReverse()),
	}

	trans := make([]*Transactions, len(block.Transactions))
	for i := 0; i < len(block.Transactions); i++ {
		trans[i] = TransArryByteToHexString(block.Transactions[i])
	}

	b := BlockInfo{
		Hash:         BytesToHexString(hash.ToArrayReverse()),
		BlockData:    blockHead,
		Transactions: trans,
	}
	return RpcResult(b)
}

func getBlockCount(params []interface{}) map[string]interface{} {
	return RpcResult(ledger.DefaultLedger.Blockchain.BlockHeight + 1)
}

// A JSON example for getblockhash method as following:
//   {"jsonrpc": "2.0", "method": "getblockhash", "params": [1], "id": 0}
func getBlockHash(params []interface{}) map[string]interface{} {
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

func getConnectionCount(params []interface{}) map[string]interface{} {
	return RpcResult(node.GetConnectionCnt())
}

func getRawMemPool(params []interface{}) map[string]interface{} {
	txs := []*Transactions{}
	txpool := node.GetTxnPool()
	for _, t := range txpool.GetAllTransactions() {
		txs = append(txs, TransArryByteToHexString(t))
	}
	if len(txs) == 0 {
		return RpcResultNil
	}
	return RpcResult(txs)
}

// A JSON example for getrawtransaction method as following:
//   {"jsonrpc": "2.0", "method": "getrawtransaction", "params": ["transactioin hash in hex"], "id": 0}
func getRawTransaction(params []interface{}) map[string]interface{} {
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
		tran := TransArryByteToHexString(tx)
		return RpcResult(tran)
	default:
		return RpcResultInvalidParameter
	}
}

// A JSON example for sendrawtransaction method as following:
//   {"jsonrpc": "2.0", "method": "sendrawtransaction", "params": ["raw transactioin in hex"], "id": 0}
func sendRawTransaction(params []interface{}) map[string]interface{} {
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
		if errCode := VerifyAndSendTx(&txn); errCode != ErrNoError {
			return RpcResult(errCode.Error())
		}
	default:
		return RpcResultInvalidParameter
	}
	return RpcResult(BytesToHexString(hash.ToArrayReverse()))
}

func getTxout(params []interface{}) map[string]interface{} {
	//TODO
	return RpcResultUnsupported
}

// A JSON example for submitblock method as following:
//   {"jsonrpc": "2.0", "method": "submitblock", "params": ["raw block in hex"], "id": 0}
func submitBlock(params []interface{}) map[string]interface{} {
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
		if err := node.LocalNode().CleanSubmittedTransactions(block.Transactions); err != nil {
			return RpcResultInternalError
		}
		if err := node.Xmit(&block); err != nil {
			return RpcResultInternalError
		}
	default:
		return RpcResultInvalidParameter
	}
	return RpcResultSuccess
}

func getNeighbor(params []interface{}) map[string]interface{} {
	addr, _ := node.GetNeighborAddrs()
	return RpcResult(addr)
}

func getNodeState(params []interface{}) map[string]interface{} {
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
	}
	return RpcResult(n)
}

func setDebugInfo(params []interface{}) map[string]interface{} {
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

func getVersion(params []interface{}) map[string]interface{} {
	return RpcResult(config.Version)
}

func getBalance(params []interface{}) map[string]interface{} {
	unspent, _ := Wallet.GetUnspent()
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

func registAsset(params []interface{}) map[string]interface{} {
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
	if Wallet == nil {
		return RpcResult("open wallet first")
	}

	txn, err := helper.MakeRegTransaction(Wallet, assetName, assetValue)
	if err != nil {
		return RpcResultInternalError
	}

	if errCode := VerifyAndSendTx(txn); errCode != ErrNoError {
		return RpcResultInvalidTransaction
	}

	txHash := txn.Hash()
	return RpcResult(BytesToHexString(txHash.ToArrayReverse()))
}

func issueAsset(params []interface{}) map[string]interface{} {
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
	if Wallet == nil {
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
	txn, err := helper.MakeIssueTransaction(Wallet, assetID, address, value)
	if err != nil {
		return RpcResultInternalError
	}

	if errCode := VerifyAndSendTx(txn); errCode != ErrNoError {
		return RpcResultInvalidTransaction
	}

	txHash := txn.Hash()
	return RpcResult(BytesToHexString(txHash.ToArrayReverse()))
}

func sendToAddress(params []interface{}) map[string]interface{} {
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
	if Wallet == nil {
		return RpcResult("error : wallet is not opened")
	}

	batchOut := helper.BatchOut{
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
	txn, err := helper.MakeTransferTransaction(Wallet, assetID, batchOut)
	if err != nil {
		return RpcResult("error: " + err.Error())
	}

	if errCode := VerifyAndSendTx(txn); errCode != ErrNoError {
		return RpcResult("error: " + errCode.Error())
	}
	txHash := txn.Hash()
	return RpcResult(BytesToHexString(txHash.ToArrayReverse()))
}

func prepaidAsset(params []interface{}) map[string]interface{} {
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
	if Wallet == nil {
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
	txn, err := helper.MakePrepaidTransaction(Wallet, assetID, assetValue, rates)
	if err != nil {
		return RpcResultInternalError
	}

	if errCode := VerifyAndSendTx(txn); errCode != ErrNoError {
		return RpcResultInvalidTransaction
	}

	txHash := txn.Hash()
	return RpcResult(BytesToHexString(txHash.ToArrayReverse()))
}

func withdrawAsset(params []interface{}) map[string]interface{} {
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
	if Wallet == nil {
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

	txn, err := helper.MakeWithdrawTransaction(Wallet, assetID, assetValue)
	if err != nil {
		return RpcResultInternalError
	}

	if errCode := VerifyAndSendTx(txn); errCode != ErrNoError {
		return RpcResultInvalidTransaction
	}

	txHash := txn.Hash()
	return RpcResult(BytesToHexString(txHash.ToArrayReverse()))
}