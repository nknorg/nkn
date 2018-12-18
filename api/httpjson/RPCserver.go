package httpjson

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/nknorg/nkn/api/common"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
)

type RPCServer struct {
	//keeps track of every function to be called on specific rpc call
	mainMux ServeMux

	//defines a slice of listeners for RPCServer, such as "127.0.0.1:30004"
	listeners []string

	//the reference of Noder
	node protocol.Noder

	//the reference of Wallet
	wallet vault.Wallet
}

type ServeMux struct {
	sync.RWMutex

	//collection of Handlers
	m map[string]common.Handler

	//will be called when the request of rpc client contains no implemented functions.
	defaultFunction func(http.ResponseWriter, *http.Request)
}

// NewServer will create a new RPC server instance.
func NewServer(node protocol.Noder, wallet vault.Wallet) *RPCServer {
	server := &RPCServer{
		mainMux: ServeMux{
			m: make(map[string]common.Handler),
		},
		listeners: []string{":" + strconv.Itoa(int(config.Parameters.HttpJsonPort))},
		node:      node,
		wallet:    wallet,
	}

	return server
}

//this is the funciton that should be called in order to answer an rpc call
//should be registered like "http.HandleFunc("/", httpjsonrpc.Handle)"
func (s *RPCServer) Handle(w http.ResponseWriter, r *http.Request) {
	s.mainMux.RLock()
	defer s.mainMux.RUnlock()
	//CORS headers
	w.Header().Add("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
	w.Header().Set("content-type", "application/json;charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "POST" {
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

		// Check the input request
		errcode := common.SUCCESS
		id, ok := request["id"].(string)
		if !ok {
			id = "1" // set default if not in request
		}
		method, ok := request["method"].(string)
		if !ok {
			errcode = common.INVALID_METHOD
		}
		params, ok := request["params"].(map[string]interface{})
		if !ok {
			errcode = common.INVALID_PARAMS
		}
		if errcode != common.SUCCESS {
			data, err := json.Marshal(map[string]interface{}{
				"jsonrpc": "2.0",
				"error": map[string]interface{}{
					"code":    -errcode,
					"message": common.ErrMessage[errcode],
				},
				"id": id,
			})
			if err != nil {
				log.Error("HTTP JSON RPC Handle - json.Marshal: ", err)
				return
			}
			w.Write(data)
			return
		}

		//get the corresponding function
		function, ok := s.mainMux.m[method]
		if ok {
			var data []byte
			var err error
			response := function(s, params)
			errcode := response["error"].(common.ErrCode)
			if errcode != common.SUCCESS {
				data, err = json.Marshal(map[string]interface{}{
					"jsonrpc": "2.0",
					"error": map[string]interface{}{
						"code":    -errcode,
						"message": common.ErrMessage[errcode],
					},
					"id": id,
				})
			} else {
				data, err = json.Marshal(map[string]interface{}{
					"jsonrpc": "2.0",
					"result":  response["result"],
					"id":      id,
				})
			}
			if err != nil {
				log.Error("HTTP JSON RPC Handle - json.Marshal: ", err)
				return
			}
			w.Write(data)
		} else {
			//if the function does not exist
			log.Warning("HTTP JSON RPC Handle - No function to call for ", method)
			errcode := common.INVALID_METHOD
			data, err := json.Marshal(map[string]interface{}{
				"jsonrpc": "2.0",
				"error": map[string]interface{}{
					"code":    -errcode,
					"message": common.ErrMessage[errcode],
				},
				"id": id,
			})
			if err != nil {
				log.Error("HTTP JSON RPC Handle - json.Marshal: ", err)
				return
			}
			w.Write(data)
		}
	}
}

//a function to register functions to be called for specific rpc calls
func (s *RPCServer) HandleFunc(pattern string, handler common.Handler) {
	s.mainMux.Lock()
	defer s.mainMux.Unlock()
	s.mainMux.m[pattern] = handler
}

//a function to be called if the request is not a HTTP JSON RPC call
func (s *RPCServer) SetDefaultFunc(def func(http.ResponseWriter, *http.Request)) {
	s.mainMux.defaultFunction = def
}

func (s *RPCServer) initTlsListen(cert, key string) (net.Listener, error) {

	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		log.Error("load keys fail", err)
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{pair},
	}

	listener, err := tls.Listen("tcp", s.listeners[0], tlsConfig)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return listener, nil
}

func (s *RPCServer) Start() {
	for name, handler := range common.InitialAPIHandlers {
		if handler.IsAccessableByJsonrpc() {
			s.HandleFunc(name, handler.Handler)
		}
	}

	var listener net.Listener
	var err error
	if config.Parameters.IsTLS {
		listener, err = s.initTlsListen(config.Parameters.RPCCert, config.Parameters.RPCKey)
		if err != nil {
			log.Error("Https Cert: ", err.Error())
			return
		}
	} else {
		listener, err = net.Listen("tcp", s.listeners[0])
		if err != nil {
			log.Error("net.Listen: ", err.Error())
			return
		}
	}

	rpcServeMux := http.NewServeMux()
	rpcServeMux.HandleFunc("/", s.Handle)
	httpServer := &http.Server{
		Handler: rpcServeMux,
	}
	httpServer.Serve(listener)
}

func (s *RPCServer) GetNetNode() (protocol.Noder, error) {
	return s.node, nil
}

func (s *RPCServer) GetWallet() (vault.Wallet, error) {
	return s.wallet, nil
}
