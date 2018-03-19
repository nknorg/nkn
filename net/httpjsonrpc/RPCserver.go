package httpjsonrpc

import (
	"nkn-core/common/log"
	"net/http"
	"strconv"
	"nkn-core/common/config"
)

func StartRPCServer() {
	log.Debug()
	http.HandleFunc("/", Handle)

	HandleFunc("getbestblockhash", getBestBlockHash)
	HandleFunc("getblock", getBlock)
	HandleFunc("getblockcount", getBlockCount)
	HandleFunc("getblockhash", getBlockHash)
	HandleFunc("getunspendoutput", getUnspendOutput)
	HandleFunc("getconnectioncount", getConnectionCount)
	HandleFunc("getrawmempool", getRawMemPool)
	HandleFunc("getrawtransaction", getRawTransaction)
	HandleFunc("sendrawtransaction", sendRawTransaction)
	HandleFunc("submitblock", submitBlock)
	HandleFunc("getversion", getVersion)
	HandleFunc("getdataile", getDataFile)
	HandleFunc("catdatarecord", catDataRecord)
	HandleFunc("regdatafile", regDataFile)
	HandleFunc("uploadDataFile", uploadDataFile)
	HandleFunc("getneighbor", getNeighbor)
	HandleFunc("getnodestate", getNodeState)
	HandleFunc("startconsensus", startConsensus)
	HandleFunc("stopconsensus", stopConsensus)
	HandleFunc("sendsampletransaction", sendSampleTransaction)
	HandleFunc("setdebuginfo", setDebugInfo)

	err := http.ListenAndServe(":"+strconv.Itoa(config.Parameters.HttpJsonPort), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err.Error())
	}
}
