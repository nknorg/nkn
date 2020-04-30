package httpjson

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/nknorg/nkn/api/common"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/node"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
	"golang.org/x/time/rate"
)

type RPCServer struct {
	// keeps track of every function to be called on specific rpc call
	mainMux       ServeMux
	httpListener  string
	httpsListener string
	localNode     *node.LocalNode
	wallet        *vault.Wallet
	limiter       *rate.Limiter
}

type ServeMux struct {
	sync.RWMutex

	// collection of Handlers
	m map[string]common.Handler

	// will be called when the request of rpc client contains no implemented functions.
	defaultFunction func(http.ResponseWriter, *http.Request)
}

// NewServer will create a new RPC server instance.
func NewServer(localNode *node.LocalNode, wallet *vault.Wallet) *RPCServer {
	return &RPCServer{
		mainMux: ServeMux{
			m: make(map[string]common.Handler),
		},
		httpListener:  ":" + strconv.Itoa(int(config.Parameters.HttpJsonPort)),
		httpsListener: ":" + strconv.Itoa(int(config.Parameters.HttpsJsonPort)),
		localNode:     localNode,
		wallet:        wallet,
		limiter:       rate.NewLimiter(rate.Limit(config.Parameters.RPCRateLimit), int(config.Parameters.RPCRateBurst)),
	}
}

// Handle is the funciton that should be called in order to answer an rpc call
// should be registered like "http.HandleFunc("/", httpjsonrpc.Handle)"
func (s *RPCServer) Handle(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.Allow() {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	defer func() {
		err := recover()
		if err != nil {
			log.Errorf("HTTP JSON RPC handler panic: %v", err)
			data, err := json.Marshal(map[string]interface{}{
				"jsonrpc": "2.0",
				"error": map[string]interface{}{
					"code":    -common.INTERNAL_ERROR,
					"message": common.ErrMessage[common.INTERNAL_ERROR],
				},
				"id": "1",
			})
			if err != nil {
				log.Error("HTTP JSON RPC JSON Marshal error: ", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(data)
		}
	}()

	s.mainMux.RLock()
	defer s.mainMux.RUnlock()
	// CORS headers
	w.Header().Add("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
	w.Header().Set("content-type", "application/json;charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "POST" {
		// read the body of the request
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
			// set default if not in request
			id = "1"
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
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Write(data)
			return
		}
		// if params["RemoteAddr"] set but empty, used request.RemoteAddr
		if addr, ok := params["RemoteAddr"]; ok {
			switch addr.(type) {
			case []byte, string:
				if len(addr.(string)) == 0 {
					params["RemoteAddr"] = r.RemoteAddr
				}
			case bool: // set remoteAddr if it's true or false
				params["RemoteAddr"] = r.RemoteAddr
			default:
				log.Warningf("RemoteAddr unsupport type for %v", addr)
			}
		}

		// get the corresponding function
		function, ok := s.mainMux.m[method]
		if ok {
			defer func() {
				err := recover()
				if err != nil {
					var errcode common.ErrCode
					if _, err = chain.GetDefaultLedger(); err != nil {
						errcode = common.ErrNullDB
					} else if s.GetNetNode() == nil {
						errcode = common.ErrNullID
					} else {
						// This panic will be recovered by handler
						panic(err)
					}
					data, err := json.Marshal(map[string]interface{}{
						"jsonrpc": "2.0",
						"error": map[string]interface{}{
							"code":    -errcode,
							"message": common.ErrMessage[errcode],
						},
						"id": id,
					})
					if err != nil {
						log.Error("HTTP JSON RPC JSON Marshal error: ", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					w.Write(data)
				}
			}()
			var data []byte
			var err error
			response := function(s, params)
			errcode := response["error"].(common.ErrCode)
			if errcode != common.SUCCESS {
				result := map[string]interface{}{
					"jsonrpc": "2.0",
					"error": map[string]interface{}{
						"code":    -errcode,
						"message": common.ErrMessage[errcode],
						"data":    response["resultOrData"],
					},
					"id": id,
				}
				data, err = json.Marshal(result)
			} else {
				data, err = json.Marshal(map[string]interface{}{
					"jsonrpc": "2.0",
					"result":  response["resultOrData"],
					"id":      id,
				})
			}
			if err != nil {
				log.Error("HTTP JSON RPC Handle - json.Marshal: ", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Write(data)
		} else {
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
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Write(data)
		}
	}
}

// function to register functions to be called for specific rpc calls
func (s *RPCServer) HandleFunc(pattern string, handler common.Handler) {
	s.mainMux.Lock()
	defer s.mainMux.Unlock()
	s.mainMux.m[pattern] = handler
}

// function to be called if the request is not a HTTP JSON RPC call
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

	listener, err := tls.Listen("tcp", s.httpsListener, tlsConfig)
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

	var listener, tlsListener net.Listener
	var err error
	tlsListener, err = s.initTlsListen(config.Parameters.HttpsJsonCert, config.Parameters.HttpsJsonKey)
	if err != nil {
		log.Error("Https Cert: ", err.Error())
		return
	}

	listener, err = net.Listen("tcp", s.httpListener)
	if err != nil {
		log.Error("net.Listen: ", err.Error())
		return
	}

	rpcServeMux := http.NewServeMux()
	rpcServeMux.HandleFunc("/", s.Handle)
	httpServer := &http.Server{
		Handler:      rpcServeMux,
		ReadTimeout:  config.Parameters.RPCReadTimeout * time.Second,
		WriteTimeout: config.Parameters.RPCWriteTimeout * time.Second,
		IdleTimeout:  config.Parameters.RPCIdleTimeout * time.Second,
	}

	httpServer.SetKeepAlivesEnabled(config.Parameters.RPCKeepAlivesEnabled)

	go httpServer.Serve(listener)
	go httpServer.Serve(tlsListener)
}

func (s *RPCServer) GetLocalNode() *node.LocalNode {
	s.mainMux.RLock()
	defer s.mainMux.RUnlock()
	return s.localNode
}

func (s *RPCServer) SetLocalNode(ln *node.LocalNode) {
	s.mainMux.Lock()
	defer s.mainMux.Unlock()
	s.localNode = ln
}

func (s *RPCServer) GetNetNode() *node.LocalNode {
	return s.GetLocalNode()
}
