package httpjson

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"

	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

const (
	LocalHost = "127.0.0.1"
)

type RPCServer struct {
	//keeps track of every function to be called on specific rpc call
	mainMux ServeMux

	//defines a slice of listeners for RPCServer, such as "127.0.0.1:30004"
	listeners []string

	//the instance of Noder
	node protocol.Noder
}

type ServeMux struct {
	sync.RWMutex

	//collection of Handlers
	m map[string]func([]interface{}) map[string]interface{}

	//will be called when the request of rpc client contains no implemented functions.
	defaultFunction func(http.ResponseWriter, *http.Request)
}

// NewServer will create a new RPC server instance.
func NewServer(node protocol.Noder) *RPCServer {
	server := &RPCServer{
		mainMux: ServeMux{
			m: make(map[string]func([]interface{}) map[string]interface{}),
		},
		listeners: []string{LocalHost + ":" + strconv.Itoa(int(config.Parameters.HttpJsonPort))},
		node:      node,
	}

	RegistRpcNode(node) //TODO delete it soon

	return server
}

//this is the funciton that should be called in order to answer an rpc call
//should be registered like "http.HandleFunc("/", httpjsonrpc.Handle)"
func (s *RPCServer) Handle(w http.ResponseWriter, r *http.Request) {
	s.mainMux.RLock()
	defer s.mainMux.RUnlock()
	//JSON RPC commands should be POSTs
	if r.Method != "POST" {
		if s.mainMux.defaultFunction != nil {
			log.Info("HTTP JSON RPC Handle - Method!=\"POST\"")
			s.mainMux.defaultFunction(w, r)
			return
		} else {
			log.Warn("HTTP JSON RPC Handle - Method!=\"POST\"")
			return
		}
	}

	//check if there is Request Body to read
	if r.Body == nil {
		if s.mainMux.defaultFunction != nil {
			log.Info("HTTP JSON RPC Handle - Request body is nil")
			s.mainMux.defaultFunction(w, r)
			return
		} else {
			log.Warn("HTTP JSON RPC Handle - Request body is nil")
			return
		}
	}

	//read the body of the request
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error("HTTP JSON RPC Handle - ioutil.ReadAll: ", err)
		return
	}
	request := make(map[string]interface{})
	err = json.Unmarshal(body, &request)
	if err != nil {
		log.Error("HTTP JSON RPC Handle - json.Unmarshal: ", err)
		return
	}

	//get the corresponding function
	function, ok := s.mainMux.m[request["method"].(string)]
	if ok {
		response := function(request["params"].([]interface{}))
		data, err := json.Marshal(map[string]interface{}{
			"jsonpc": "2.0",
			"result": response["result"],
			"id":     request["id"],
		})
		if err != nil {
			log.Error("HTTP JSON RPC Handle - json.Marshal: ", err)
			return
		}
		w.Write(data)
	} else {
		//if the function does not exist
		log.Warn("HTTP JSON RPC Handle - No function to call for ", request["method"])
		data, err := json.Marshal(map[string]interface{}{
			"result": nil,
			"error": map[string]interface{}{
				"code":    -32601,
				"message": "Method not found",
				"data":    "The called method was not found on the server",
			},
			"id": request["id"],
		})
		if err != nil {
			log.Error("HTTP JSON RPC Handle - json.Marshal: ", err)
			return
		}
		w.Write(data)
	}
}

//a function to register functions to be called for specific rpc calls
func (s *RPCServer) HandleFunc(pattern string, handler func([]interface{}) map[string]interface{}) {
	s.mainMux.Lock()
	defer s.mainMux.Unlock()
	s.mainMux.m[pattern] = handler
}

//a function to be called if the request is not a HTTP JSON RPC call
func (s *RPCServer) SetDefaultFunc(def func(http.ResponseWriter, *http.Request)) {
	s.mainMux.defaultFunction = def
}

func (s *RPCServer) Start() {
	log.Debug()
	http.HandleFunc("/", s.Handle)

	s.HandleFunc("getbestblockhash", getBestBlockHash)
	s.HandleFunc("getblock", getBlock)
	s.HandleFunc("getblockcount", getBlockCount)
	s.HandleFunc("getblockhash", getBlockHash)
	s.HandleFunc("getconnectioncount", getConnectionCount)
	s.HandleFunc("getrawmempool", getRawMemPool)
	s.HandleFunc("getrawtransaction", getRawTransaction)
	s.HandleFunc("sendrawtransaction", sendRawTransaction)
	s.HandleFunc("getversion", getVersion)
	s.HandleFunc("getneighbor", getNeighbor)
	s.HandleFunc("getnodestate", getNodeState)
	s.HandleFunc("getbalance", getBalance)

	s.HandleFunc("setdebuginfo", setDebugInfo)
	s.HandleFunc("sendtoaddress", sendToAddress)
	s.HandleFunc("registasset", registAsset)
	s.HandleFunc("issueasset", issueAsset)
	s.HandleFunc("prepaidasset", prepaidAsset)
	s.HandleFunc("withdrawasset", withdrawAsset)
	s.HandleFunc("commitpor", commitPor)

	err := http.ListenAndServe(s.listeners[0], nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err.Error())
	}
}
