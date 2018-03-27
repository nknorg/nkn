package httpjsonrpc

import (
	"nkn-core/common/config"
	"nkn-core/common/log"
	"net/http"
	"strconv"
)

const (
	LocalHost = "127.0.0.1"
)

func StartRPCServer() {
	log.Debug()
	http.HandleFunc("/", Handle)

	HandleFunc("getbestblockhash", getBestBlockHash)
	HandleFunc("getblock", getBlock)
	HandleFunc("getblockcount", getBlockCount)
	HandleFunc("getblockhash", getBlockHash)
	HandleFunc("getconnectioncount", getConnectionCount)
	HandleFunc("getrawmempool", getRawMemPool)
	HandleFunc("getrawtransaction", getRawTransaction)
	HandleFunc("sendrawtransaction", sendRawTransaction)
	HandleFunc("getversion", getVersion)
	HandleFunc("getneighbor", getNeighbor)
	HandleFunc("getnodestate", getNodeState)
	HandleFunc("getbalance", getBalance)


	HandleFunc("setdebuginfo", setDebugInfo)
	HandleFunc("setdebuginfo", sendToAddress)
	HandleFunc("registasset", registAsset)
	HandleFunc("issueasset", issueAsset)
	HandleFunc("prepaidasset", prepaidAsset)
	HandleFunc("withdrawasset", withdrawAsset)

	err := http.ListenAndServe(LocalHost+":"+strconv.Itoa(config.Parameters.HttpJsonPort), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err.Error())
	}
}