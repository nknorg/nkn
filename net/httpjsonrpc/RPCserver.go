package httpjsonrpc

import (
	"net/http"
	"nkn-core/common/config"
	"nkn-core/common/log"
	"strconv"
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
	HandleFunc("submitblock", submitBlock)
	HandleFunc("getversion", getVersion)
	HandleFunc("getneighbor", getNeighbor)
	HandleFunc("getnodestate", getNodeState)
	HandleFunc("setdebuginfo", setDebugInfo)

	err := http.ListenAndServe(":"+strconv.Itoa(config.Parameters.HttpJsonPort), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err.Error())
	}
}
