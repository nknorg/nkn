package httpjson

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"

	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
)

type RPCServer struct {
	//keeps track of every function to be called on specific rpc call
	mainMux ServeMux

	//defines a slice of listeners for RPCServer, such as "127.0.0.1:30004"
	listeners []string

	//the reference of Noder
	node protocol.Noder

	//the reference of Wallet
	wallet wallet.Wallet
}

type funcHandler func(*RPCServer, []interface{}) map[string]interface{}

type ServeMux struct {
	sync.RWMutex

	//collection of Handlers
	m map[string]funcHandler

	//will be called when the request of rpc client contains no implemented functions.
	defaultFunction func(http.ResponseWriter, *http.Request)
}

// NewServer will create a new RPC server instance.
func NewServer(node protocol.Noder, wallet wallet.Wallet) *RPCServer {
	server := &RPCServer{
		mainMux: ServeMux{
			m: make(map[string]funcHandler),
		},
		listeners: []string{":" + strconv.Itoa(int(config.Parameters.HttpJsonPort))},
		node:      node,
		wallet:    wallet,
	}

	return server
}

func (s *RPCServer) write(w http.ResponseWriter, data []byte) {
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("content-type", "application/json;charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(data)
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
		response := function(s, request["params"].([]interface{}))
		data, err := json.Marshal(map[string]interface{}{
			"jsonpc": "2.0",
			"result": response["result"],
			"id":     request["id"],
		})
		if err != nil {
			log.Error("HTTP JSON RPC Handle - json.Marshal: ", err)
			return
		}
		s.write(w, data)
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
		s.write(w, data)
	}
}

//a function to register functions to be called for specific rpc calls
func (s *RPCServer) HandleFunc(pattern string, handler funcHandler) {
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

	for name, handler := range initialRPCHandlers {
		s.HandleFunc(name, handler)
	}

	err := http.ListenAndServe(s.listeners[0], nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err.Error())
	}
}

func (s *RPCServer) VerifyAndSendTx(txn *transaction.Transaction) errors.ErrCode {
	// if transaction is verified unsucessfully then will not put it into transaction pool
	if errCode := s.node.AppendTxnPool(txn); errCode != errors.ErrNoError {
		log.Warn("Can NOT add the transaction to TxnPool")
		log.Info("[httpjsonrpc] VerifyTransaction failed when AppendTxnPool.")
		return errCode
	}
	if err := s.node.Xmit(txn); err != nil {
		log.Error("Xmit Tx Error:Xmit transaction failed.", err)
		return errors.ErrXmitFail
	}
	return errors.ErrNoError
}
