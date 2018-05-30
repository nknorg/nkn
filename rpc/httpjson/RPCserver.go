package httpjson

import (
	"net/http"
	"strconv"

	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
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
	HandleFunc("getwsaddr", getWsAddr)

	HandleFunc("setdebuginfo", setDebugInfo)
	HandleFunc("sendtoaddress", sendToAddress)
	HandleFunc("registasset", registAsset)
	HandleFunc("issueasset", issueAsset)
	HandleFunc("prepaidasset", prepaidAsset)
	HandleFunc("withdrawasset", withdrawAsset)
	HandleFunc("commitpor", commitPor)
	HandleFunc("sigchaintest", sigchaintest)

	err := http.ListenAndServe(LocalHost+":"+strconv.Itoa(int(config.Parameters.HttpJsonPort)), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err.Error())
	}
}

func StartServer(n protocol.Noder) {
	RegistRpcNode(n)
	go StartRPCServer()
}
