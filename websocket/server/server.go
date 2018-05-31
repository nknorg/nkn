package server

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/net/protocol"
	. "github.com/nknorg/nkn/rpc/httprestful/common"
	Err "github.com/nknorg/nkn/rpc/httprestful/error"
	"github.com/nknorg/nkn/util/address"
	. "github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	. "github.com/nknorg/nkn/websocket/session"

	"github.com/gorilla/websocket"
)

type handler func(map[string]interface{}) map[string]interface{}
type Handler struct {
	handler  handler
	pushFlag bool
}

type WsServer struct {
	sync.RWMutex
	Upgrader    websocket.Upgrader
	listener    net.Listener
	server      *http.Server
	SessionList *SessionList
	ActionMap   map[string]Handler
	TxHashMap   map[string]string //key: txHash   value:sessionid
	node        protocol.Noder
}

func InitWsServer(node protocol.Noder) *WsServer {
	ws := &WsServer{
		Upgrader:    websocket.Upgrader{},
		SessionList: NewSessionList(),
		TxHashMap:   make(map[string]string),
		node:        node,
	}
	return ws
}

func (ws *WsServer) Start() error {
	if Parameters.HttpWsPort == 0 {
		log.Error("Not configure HttpWsPort port ")
		return nil
	}
	ws.registryMethod()
	ws.Upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	tlsFlag := false
	if tlsFlag || Parameters.HttpWsPort%1000 == TlsPort {
		var err error
		ws.listener, err = ws.initTlsListen()
		if err != nil {
			log.Error("Https Cert: ", err.Error())
			return err
		}
	} else {
		var err error
		ws.listener, err = net.Listen("tcp", ":"+strconv.Itoa(int(Parameters.HttpWsPort)))
		if err != nil {
			log.Fatal("net.Listen: ", err.Error())
			return err
		}
	}
	var done = make(chan bool)
	go ws.checkSessionsTimeout(done)

	ws.server = &http.Server{Handler: http.HandlerFunc(ws.webSocketHandler)}
	err := ws.server.Serve(ws.listener)

	done <- true
	if err != nil {
		log.Fatal("ListenAndServe: ", err.Error())
		return err
	}
	return nil

}

func (ws *WsServer) registryMethod() {
	gettxhashmap := func(cmd map[string]interface{}) map[string]interface{} {
		resp := ResponsePack(Err.SUCCESS)
		ws.Lock()
		defer ws.Unlock()
		resp["Result"] = len(ws.TxHashMap)
		return resp
	}
	sendRawTransaction := func(cmd map[string]interface{}) map[string]interface{} {
		resp := SendRawTransaction(cmd)
		if userid, ok := resp["Userid"].(string); ok && len(userid) > 0 {
			if result, ok := resp["Result"].(string); ok {
				ws.SetTxHashMap(result, userid)
			}
			delete(resp, "Userid")
		}
		return resp
	}
	heartbeat := func(cmd map[string]interface{}) map[string]interface{} {
		resp := ResponsePack(Err.SUCCESS)
		resp["Action"] = "heartbeat"
		resp["Result"] = cmd["Userid"]
		return resp
	}
	sendtest := func(cmd map[string]interface{}) map[string]interface{} {
		go func() {
			time.Sleep(time.Second * 5)
			resp := ResponsePack(Err.SUCCESS)
			resp["Action"] = "pushresult"
			ws.PushTxResult(cmd["Userid"].(string), resp)
		}()
		return heartbeat(cmd)
	}
	getsessioncount := func(cmd map[string]interface{}) map[string]interface{} {
		resp := ResponsePack(Err.SUCCESS)
		resp["Action"] = "getsessioncount"
		resp["Result"] = ws.SessionList.GetSessionCount()
		return resp
	}
	setClient := func(cmd map[string]interface{}) map[string]interface{} {
		addrStr, ok := cmd["Addr"].(string)
		if !ok {
			return ResponsePack(Err.INVALID_PARAMS)
		}
		clientID, pubKey, err := address.ParseClientAddress(addrStr)
		if err != nil {
			log.Error("Parse client address error:", err)
			return ResponsePack(Err.INVALID_PARAMS)
		}

		_, err = crypto.DecodePoint(pubKey)
		if err != nil {
			log.Error("Invalid public key hex decoding to point:", err)
			return ResponsePack(Err.INVALID_PARAMS)
		}

		// TODO: use signature (or better, with one-time challange) to verify identity

		nextHop, err := ws.node.NextHop(clientID)
		if err != nil {
			log.Error("Get next hop error: ", err)
			return ResponsePack(Err.INTERNAL_ERROR)
		}
		if nextHop != nil {
			log.Error("This is not the correct node to connect")
			return ResponsePack(Err.INVALID_PARAMS)
		}

		newSessionId := hex.EncodeToString(clientID)
		err = ws.SessionList.ChangeSessionId(cmd["Userid"].(string), newSessionId)
		if err != nil {
			log.Error("Change session id error: ", err)
			return ResponsePack(Err.INTERNAL_ERROR)
		}
		session := ws.SessionList.GetSessionById(newSessionId)
		if session == nil {
			log.Error("Nil session with id: ", newSessionId)
			return ResponsePack(Err.INTERNAL_ERROR)
		}
		session.SetClient(clientID, pubKey, &addrStr)
		resp := ResponsePack(Err.SUCCESS)
		return resp
	}
	relayHandler := func(cmd map[string]interface{}) map[string]interface{} {
		client := ws.SessionList.GetSessionById(cmd["Userid"].(string))
		if !client.IsClient() {
			log.Error("Session is not client")
			return ResponsePack(Err.INVALID_METHOD)
		}
		srcAddrStr := client.GetAddrStr()
		srcPubkey := client.GetPubKey()
		if srcPubkey == nil {
			log.Error("Session does not have a public key")
			return ResponsePack(Err.INVALID_METHOD)
		}
		addrStr, ok := cmd["Dest"].(string)
		if !ok {
			return ResponsePack(Err.INVALID_PARAMS)
		}
		destID, destPubkey, err := address.ParseClientAddress(addrStr)
		if err != nil {
			log.Error("Parse client address error:", err)
			return ResponsePack(Err.INVALID_PARAMS)
		}
		payload := []byte(cmd["Payload"].(string))
		signature, err := hex.DecodeString(cmd["Signature"].(string))
		if err != nil {
			log.Error("Decode signature error:", err)
			return ResponsePack(Err.INVALID_PARAMS)
		}
		err = ws.node.SendRelayPacket([]byte(*srcAddrStr), srcPubkey, destID[:], destPubkey, payload, signature)
		if err != nil {
			log.Error("Send relay packet error:", err)
			return ResponsePack(Err.INTERNAL_ERROR)
		}
		resp := ResponsePack(Err.SUCCESS)
		return resp
	}
	actionMap := map[string]Handler{
		"getconnectioncount": {handler: GetConnectionCount},
		"getblockbyheight":   {handler: GetBlockByHeight},
		"getblockbyhash":     {handler: GetBlockByHash},
		"getblockheight":     {handler: GetBlockHeight},
		"gettransaction":     {handler: GetTransactionByHash},
		"getasset":           {handler: GetAssetByHash},
		"getunspendoutput":   {handler: GetUnspendOutput},

		"sendrawtransaction": {handler: sendRawTransaction},
		"sendrecord":         {handler: SendRecord},
		"heartbeat":          {handler: heartbeat},

		"sendtest": {handler: sendtest, pushFlag: true},

		"gettxhashmap":    {handler: gettxhashmap},
		"getsessioncount": {handler: getsessioncount},

		"setClient":  {handler: setClient},
		"sendPacket": {handler: relayHandler},
	}
	ws.ActionMap = actionMap
}

func (ws *WsServer) Stop() {
	if ws.server != nil {
		ws.server.Shutdown(context.Background())
		log.Error("Close websocket ")
	}
}

func (ws *WsServer) Restart() {
	go func() {
		time.Sleep(time.Second)
		ws.Stop()
		time.Sleep(time.Second)
		go ws.Start()
	}()
}

func (ws *WsServer) checkSessionsTimeout(done chan bool) {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			var closeList []*Session
			ws.SessionList.ForEachSession(func(v *Session) {
				if v.SessionTimeoverCheck() {
					resp := ResponsePack(Err.SESSION_EXPIRED)
					ws.response(v.GetSessionId(), resp)
					closeList = append(closeList, v)
				}
			})
			for _, s := range closeList {
				ws.SessionList.CloseSession(s)
			}

		case <-done:
			return
		}
	}

}

//webSocketHandler
func (ws *WsServer) webSocketHandler(w http.ResponseWriter, r *http.Request) {
	wsConn, err := ws.Upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Error("websocket Upgrader: ", err)
		return
	}
	defer wsConn.Close()
	nsSession, err := ws.SessionList.NewSession(wsConn)
	if err != nil {
		log.Error("websocket NewSession:", err)
		return
	}

	defer func() {
		ws.deleteTxHashs(nsSession.GetSessionId())
		ws.SessionList.CloseSession(nsSession)
		if err := recover(); err != nil {
			log.Fatal("websocket recover:", err)
		}
	}()

	for {
		_, bysMsg, err := wsConn.ReadMessage()
		if err == nil {
			if ws.OnDataHandle(nsSession, bysMsg, r) {
				nsSession.UpdateActiveTime()
			}
			continue
		}
		e, ok := err.(net.Error)
		if !ok || !e.Timeout() {
			log.Error("websocket conn:", err)
			return
		}
	}
}

func (ws *WsServer) IsValidMsg(reqMsg map[string]interface{}) bool {
	if _, ok := reqMsg["Hash"].(string); !ok && reqMsg["Hash"] != nil {
		return false
	}
	if _, ok := reqMsg["Addr"].(string); !ok && reqMsg["Addr"] != nil {
		return false
	}
	if _, ok := reqMsg["Assetid"].(string); !ok && reqMsg["Assetid"] != nil {
		return false
	}
	return true
}

func (ws *WsServer) OnDataHandle(curSession *Session, bysMsg []byte, r *http.Request) bool {

	var req = make(map[string]interface{})

	if err := json.Unmarshal(bysMsg, &req); err != nil {
		resp := ResponsePack(Err.ILLEGAL_DATAFORMAT)
		ws.response(curSession.GetSessionId(), resp)
		log.Error("websocket OnDataHandle:", err)
		return false
	}
	actionName, ok := req["Action"].(string)
	if !ok {
		resp := ResponsePack(Err.INVALID_METHOD)
		ws.response(curSession.GetSessionId(), resp)
		return false
	}
	action, ok := ws.ActionMap[actionName]
	if !ok {
		resp := ResponsePack(Err.INVALID_METHOD)
		ws.response(curSession.GetSessionId(), resp)
		return false
	}
	if !ws.IsValidMsg(req) {
		resp := ResponsePack(Err.INVALID_PARAMS)
		ws.response(curSession.GetSessionId(), resp)
		return true
	}
	if height, ok := req["Height"].(float64); ok {
		req["Height"] = strconv.FormatInt(int64(height), 10)
	}
	if raw, ok := req["Raw"].(float64); ok {
		req["Raw"] = strconv.FormatInt(int64(raw), 10)
	}
	req["Userid"] = curSession.GetSessionId()
	resp := action.handler(req)
	resp["Action"] = actionName
	if txHash, ok := resp["Result"].(string); ok && action.pushFlag {
		ws.Lock()
		defer ws.Unlock()
		ws.TxHashMap[txHash] = curSession.GetSessionId()
	}
	ws.response(curSession.GetSessionId(), resp)

	return true
}

func (ws *WsServer) SetTxHashMap(txhash string, sessionid string) {
	ws.Lock()
	defer ws.Unlock()
	ws.TxHashMap[txhash] = sessionid
}

func (ws *WsServer) deleteTxHashs(sSessionId string) {
	ws.Lock()
	defer ws.Unlock()
	for k, v := range ws.TxHashMap {
		if v == sSessionId {
			delete(ws.TxHashMap, k)
		}
	}
}

func (ws *WsServer) response(sSessionId string, resp map[string]interface{}) {
	resp["Desc"] = Err.ErrMap[resp["Error"].(int64)]
	data, err := json.Marshal(resp)
	if err != nil {
		log.Error("Websocket response:", err)
		return
	}
	ws.send(sSessionId, data)
}

func (ws *WsServer) PushTxResult(txHashStr string, resp map[string]interface{}) {
	ws.Lock()
	defer ws.Unlock()
	sSessionId := ws.TxHashMap[txHashStr]
	delete(ws.TxHashMap, txHashStr)
	if len(sSessionId) > 0 {
		ws.response(sSessionId, resp)
	}
	ws.PushResult(resp)
}

func (ws *WsServer) PushResult(resp map[string]interface{}) {
	resp["Desc"] = Err.ErrMap[resp["Error"].(int64)]
	data, err := json.Marshal(resp)
	if err != nil {
		log.Error("Websocket PushResult:", err)
		return
	}
	ws.Broadcast(data)
}

func (ws *WsServer) send(sSessionId string, data []byte) error {
	session := ws.SessionList.GetSessionById(sSessionId)
	if session == nil {
		return errors.New("websocket sessionId Not Exist: " + sSessionId)
	}
	return session.Send(data)
}

func (ws *WsServer) Broadcast(data []byte) error {
	// TODO: only send to subscribed sessions
	ws.SessionList.ForEachSession(func(v *Session) {
		v.Send(data)
	})
	return nil
}

func (ws *WsServer) initTlsListen() (net.Listener, error) {

	CertPath := Parameters.RestCertPath
	KeyPath := Parameters.RestKeyPath

	// load cert
	cert, err := tls.LoadX509KeyPair(CertPath, KeyPath)
	if err != nil {
		log.Error("load keys fail", err)
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	log.Info("TLS listen port is ", strconv.Itoa(int(Parameters.HttpWsPort)))
	listener, err := tls.Listen("tcp", ":"+strconv.Itoa(int(Parameters.HttpWsPort)), tlsConfig)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return listener, nil
}

func (ws *WsServer) GetClientById(cliendID []byte) *Session {
	session := ws.SessionList.GetSessionById(hex.EncodeToString(cliendID))
	if session == nil {
		return nil
	}
	if !session.IsClient() {
		return nil
	}
	return session
}
