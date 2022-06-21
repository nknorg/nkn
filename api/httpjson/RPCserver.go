package httpjson

import (
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/nknorg/nkn/v2/api/common"
	"github.com/nknorg/nkn/v2/api/common/errcode"
	"github.com/nknorg/nkn/v2/api/ratelimiter"
	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/node"
	"github.com/nknorg/nkn/v2/util/log"
	"github.com/nknorg/nkn/v2/vault"
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
	httpServer    *http.Server
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
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		limiter := ratelimiter.GetLimiter("rpc:"+host, config.Parameters.RPCIPRateLimit, int(config.Parameters.RPCIPRateBurst))
		if !limiter.Allow() {
			log.Infof("RPC connection limit of %s reached", host)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
	}

	if !s.limiter.Allow() {
		log.Infof("RPC connection limit reached")
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
					"code":    -errcode.INTERNAL_ERROR,
					"message": errcode.ErrMessage[errcode.INTERNAL_ERROR],
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
		defer r.Body.Close()

		// read the body of the request
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Error("HTTP JSON RPC Handle - ioutil.ReadAll: ", err)
			return
		}

		var code errcode.ErrCode
		var id, method string
		var params map[string]interface{}

		request := make(map[string]interface{})
		err = json.Unmarshal(body, &request)
		if err == nil {
			code = errcode.SUCCESS
			var ok bool
			id, ok = request["id"].(string)
			if !ok {
				// set default if not in request
				id = "1"
			}
			method, ok = request["method"].(string)
			if !ok {
				code = errcode.INVALID_METHOD
			}
			if request["params"] != nil {
				params, ok = request["params"].(map[string]interface{})
				if !ok {
					code = errcode.INVALID_PARAMS
				}
			}
		} else {
			code = errcode.INVALID_JSON
		}

		if code != errcode.SUCCESS {
			data, err := json.Marshal(map[string]interface{}{
				"jsonrpc": "2.0",
				"error": map[string]interface{}{
					"code":    -code,
					"message": errcode.ErrMessage[code],
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
					var code errcode.ErrCode
					if _, err = chain.GetDefaultLedger(); err != nil {
						code = errcode.ErrNullDB
					} else if s.GetNetNode() == nil {
						code = errcode.ErrNullID
					} else {
						// This panic will be recovered by handler
						panic(err)
					}
					data := map[string]interface{}{
						"jsonrpc": "2.0",
						"error": map[string]interface{}{
							"code":    -code,
							"message": errcode.ErrMessage[code],
						},
						"id": id,
					}
					if code == errcode.ErrNullID {
						acc, err := s.wallet.GetDefaultAccount()
						if err != nil {
							log.Error("HTTP JSON RPC GetDefaultAccount error: ", err)
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						pubkey := acc.PubKey()
						data["error"].(map[string]interface{})["publicKey"] = hex.EncodeToString(pubkey)
						walletAddress, err := acc.ProgramHash.ToAddress()
						if err != nil {
							log.Error("HTTP JSON RPC ProgramHash ToAddress error: ", err)
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						data["error"].(map[string]interface{})["walletAddress"] = walletAddress
					}
					jsonData, err := json.Marshal(data)
					if err != nil {
						log.Error("HTTP JSON RPC JSON Marshal error: ", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					w.Write(jsonData)
				}
			}()
			var data []byte
			var err error
			response := function(s, params, r.Context())
			code := response["error"].(errcode.ErrCode)
			if code != errcode.SUCCESS {
				result := map[string]interface{}{
					"jsonrpc": "2.0",
					"error": map[string]interface{}{
						"code":    -code,
						"message": errcode.ErrMessage[code],
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
			code := errcode.INVALID_METHOD
			data, err := json.Marshal(map[string]interface{}{
				"jsonrpc": "2.0",
				"error": map[string]interface{}{
					"code":    -code,
					"message": errcode.ErrMessage[code],
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

func (s *RPCServer) initTlsListen() (net.Listener, error) {
	tlsConfig := &tls.Config{
		GetCertificate: common.GetHttpsCertificate,
	}

	listener, err := tls.Listen("tcp", s.httpsListener, tlsConfig)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return listener, nil
}

func (s *RPCServer) Start(httpsCertReady chan struct{}) {
	for name, handler := range common.InitialAPIHandlers {
		if handler.IsAccessableByJsonrpc() {
			s.HandleFunc(name, handler.Handler)
		}
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
	s.httpServer = httpServer
	listener, err := net.Listen("tcp", s.httpListener)
	if err != nil {
		log.Error("net.Listen: ", err.Error())
		return
	}
	go s.httpServer.Serve(listener)

	go func(httpsCertReady chan struct{}) {
		for {
			select {
			case <-httpsCertReady:
				log.Info("https cert received")
				tlsListener, err := s.initTlsListen()
				if err != nil {
					log.Errorf("Https Cert: %v", err.Error())
					return
				}
				err = s.httpServer.Serve(tlsListener)
				if err != nil {
					log.Error(err)
				}
				return
			case <-time.After(300 * time.Second):
				log.Info("https server is unavailable yet")
			}
		}
	}(httpsCertReady)
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
