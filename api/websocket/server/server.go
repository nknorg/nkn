package server

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/api/common"
	"github.com/nknorg/nkn/api/websocket/messagebuffer"
	"github.com/nknorg/nkn/api/websocket/session"
	"github.com/nknorg/nkn/chain"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/event"
	"github.com/nknorg/nkn/node"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"

	"github.com/gorilla/websocket"
)

const (
	TlsPort                      uint16 = 443
	sigChainCacheExpiration             = config.ConsensusTimeout
	sigChainCacheCleanupInterval        = time.Second
)

type Handler struct {
	handler  common.Handler
	pushFlag bool
}

type WsServer struct {
	sync.RWMutex
	Upgrader      websocket.Upgrader
	listener      net.Listener
	server        *http.Server
	SessionList   *session.SessionList
	ActionMap     map[string]Handler
	TxHashMap     map[string]string //key: txHash   value:sessionid
	localNode     *node.LocalNode
	wallet        vault.Wallet
	messageBuffer *messagebuffer.MessageBuffer
	sigChainCache Cache
}

func InitWsServer(localNode *node.LocalNode, wallet vault.Wallet) *WsServer {
	ws := &WsServer{
		Upgrader:      websocket.Upgrader{},
		SessionList:   session.NewSessionList(),
		TxHashMap:     make(map[string]string),
		localNode:     localNode,
		wallet:        wallet,
		messageBuffer: messagebuffer.NewMessageBuffer(),
		sigChainCache: NewGoCache(sigChainCacheExpiration, sigChainCacheCleanupInterval),
	}
	return ws
}

func (ws *WsServer) Start() error {
	if config.Parameters.HttpWsPort == 0 {
		log.Error("Not configure HttpWsPort port ")
		return nil
	}
	ws.registryMethod()
	ws.Upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	tlsFlag := false
	if tlsFlag || config.Parameters.HttpWsPort%1000 == TlsPort {
		var err error
		ws.listener, err = ws.initTlsListen()
		if err != nil {
			log.Error("Https Cert: ", err.Error())
			return err
		}
	} else {
		var err error
		ws.listener, err = net.Listen("tcp", ":"+strconv.Itoa(int(config.Parameters.HttpWsPort)))
		if err != nil {
			log.Error("net.Listen: ", err.Error())
			return err
		}
	}

	event.Queue.Subscribe(event.SendInboundMessageToClient, ws.sendInboundRelayMessageToClient)

	var done = make(chan bool)
	go ws.checkSessionsTimeout(done)

	ws.server = &http.Server{Handler: http.HandlerFunc(ws.websocketHandler)}
	err := ws.server.Serve(ws.listener)

	done <- true
	if err != nil {
		log.Error("ListenAndServe: ", err.Error())
		return err
	}
	return nil

}

func (ws *WsServer) registryMethod() {
	gettxhashmap := func(s common.Serverer, cmd map[string]interface{}) map[string]interface{} {
		ws.Lock()
		defer ws.Unlock()
		resp := common.RespPacking(len(ws.TxHashMap), common.SUCCESS)
		return resp
	}

	heartbeat := func(s common.Serverer, cmd map[string]interface{}) map[string]interface{} {
		return common.RespPacking(cmd["Userid"], common.SUCCESS)

	}

	getsessioncount := func(s common.Serverer, cmd map[string]interface{}) map[string]interface{} {
		return common.RespPacking(ws.SessionList.GetSessionCount(), common.SUCCESS)
	}

	setClient := func(s common.Serverer, cmd map[string]interface{}) map[string]interface{} {
		addrStr, ok := cmd["Addr"].(string)
		if !ok {
			return common.RespPacking(nil, common.INVALID_PARAMS)
		}
		clientID, pubKey, _, err := address.ParseClientAddress(addrStr)
		if err != nil {
			log.Error("Parse client address error:", err)
			return common.RespPacking(nil, common.INVALID_PARAMS)
		}

		_, err = crypto.DecodePoint(pubKey)
		if err != nil {
			log.Error("Invalid public key hex decoding to point:", err)
			return common.RespPacking(nil, common.INVALID_PARAMS)
		}

		// TODO: use signature (or better, with one-time challange) to verify identity

		localNode, err := s.GetNetNode()
		if err != nil {
			return common.RespPacking(nil, common.INTERNAL_ERROR)
		}

		addr, pubkey, id, err := localNode.FindWsAddr(clientID)
		if err != nil {
			log.Errorf("Find websocket address error: %v", err)
			return common.RespPacking(nil, common.INTERNAL_ERROR)
		}

		if addr != localNode.GetWsAddr() {
			return common.RespPacking(common.NodeInfo(addr, pubkey, id), common.WRONG_NODE)
		}

		newSessionID := hex.EncodeToString(clientID)
		session, err := ws.SessionList.ChangeSessionToClient(cmd["Userid"].(string), newSessionID)
		if err != nil {
			log.Error("Change session id error: ", err)
			return common.RespPacking(nil, common.INTERNAL_ERROR)
		}
		session.SetClient(clientID, pubKey, &addrStr)

		go func() {
			messages := ws.messageBuffer.PopMessages(clientID)
			for _, message := range messages {
				ws.sendInboundRelayMessage(message)
			}
		}()

		var sigChainBlockHeight uint32
		if chain.DefaultLedger.Store.GetHeight() >= config.SigChainBlockDelay {
			sigChainBlockHeight = chain.DefaultLedger.Store.GetHeight() - config.SigChainBlockDelay
		}
		sigChainBlockHash, err := chain.DefaultLedger.Store.GetBlockHash(sigChainBlockHeight)
		if err != nil {
			log.Warningf("get sigchain block hash at height %d error: %v", sigChainBlockHeight, err)
		}

		res := make(map[string]interface{})
		res["node"] = common.NodeInfo(addr, pubkey, id)
		res["sigChainBlockHash"] = BytesToHexString(sigChainBlockHash.ToArray())

		return common.RespPacking(res, common.SUCCESS)
	}

	actionMap := map[string]Handler{
		"heartbeat":       {handler: heartbeat},
		"gettxhashmap":    {handler: gettxhashmap},
		"getsessioncount": {handler: getsessioncount},
		"setClient":       {handler: setClient},
	}

	for name, handler := range common.InitialAPIHandlers {
		if handler.IsAccessableByWebsocket() {
			actionMap[name] = Handler{handler: handler.Handler}
		}
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
			var closeList []*session.Session
			ws.SessionList.ForEachSession(func(s *session.Session) {
				if s.SessionTimeoverCheck() {
					resp := common.ResponsePack(common.SESSION_EXPIRED)
					ws.respondToSession(s, resp)
					closeList = append(closeList, s)
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

//websocketHandler
func (ws *WsServer) websocketHandler(w http.ResponseWriter, r *http.Request) {
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
			log.Error("websocket recover:", err)
		}
	}()

	for {
		messageType, bysMsg, err := wsConn.ReadMessage()
		if err != nil {
			log.Debugf("websocket read message error: %v", err)
			break
		}

		if ws.OnDataHandle(nsSession, messageType, bysMsg, r) {
			nsSession.UpdateActiveTime()
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

func (ws *WsServer) OnDataHandle(curSession *session.Session, messageType int, bysMsg []byte, r *http.Request) bool {
	if messageType == websocket.BinaryMessage {
		msg := &pb.ClientMessage{}
		err := proto.Unmarshal(bysMsg, msg)
		if err != nil {
			log.Error("Parse client message error:", err)
			return false
		}

		switch msg.MessageType {
		case pb.OUTBOUND_MESSAGE:
			outboundMsg := &pb.OutboundMessage{}
			err = proto.Unmarshal(msg.Message, outboundMsg)
			if err != nil {
				log.Errorf("Unmarshal outbound message error: %v", err)
				return false
			}
			ws.sendOutboundRelayMessage(curSession.GetAddrStr(), outboundMsg)
		case pb.RECEIPT:
			receipt := &pb.Receipt{}
			err = proto.Unmarshal(msg.Message, receipt)
			if err != nil {
				log.Errorf("Unmarshal receipt error: %v", err)
				return false
			}
			err = ws.handleReceipt(receipt)
			if err != nil {
				log.Errorf("Handle receipt error: %v", err)
				return false
			}
		default:
			log.Errorf("unsupported client message type %v", msg.MessageType)
			return false
		}

		return true
	}

	var req = make(map[string]interface{})

	if err := json.Unmarshal(bysMsg, &req); err != nil {
		resp := common.ResponsePack(common.ILLEGAL_DATAFORMAT)
		ws.respondToSession(curSession, resp)
		log.Error("websocket OnDataHandle:", err)
		return false
	}
	actionName, ok := req["Action"].(string)
	if !ok {
		resp := common.ResponsePack(common.INVALID_METHOD)
		ws.respondToSession(curSession, resp)
		return false
	}
	action, ok := ws.ActionMap[actionName]
	if !ok {
		resp := common.ResponsePack(common.INVALID_METHOD)
		ws.respondToSession(curSession, resp)
		return false
	}
	if !ws.IsValidMsg(req) {
		resp := common.ResponsePack(common.INVALID_PARAMS)
		ws.respondToSession(curSession, resp)
		return true
	}
	if height, ok := req["Height"].(float64); ok {
		req["Height"] = strconv.FormatInt(int64(height), 10)
	}
	if raw, ok := req["Raw"].(float64); ok {
		req["Raw"] = strconv.FormatInt(int64(raw), 10)
	}
	req["Userid"] = curSession.GetSessionId()
	ret := action.handler(ws, req)
	resp := common.ResponsePack(ret["error"].(common.ErrCode))
	resp["Action"] = actionName
	resp["Result"] = ret["resultOrData"]
	if txHash, ok := resp["Result"].(string); ok && action.pushFlag {
		ws.Lock()
		defer ws.Unlock()
		ws.TxHashMap[txHash] = curSession.GetSessionId()
	}
	ws.respondToSession(curSession, resp)

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

func (ws *WsServer) respondToSession(session *session.Session, resp map[string]interface{}) {
	resp["Desc"] = common.ErrMessage[resp["Error"].(common.ErrCode)]
	data, err := json.Marshal(resp)
	if err != nil {
		log.Error("Websocket response:", err)
		return
	}
	session.SendText(data)
}

func (ws *WsServer) respondToId(sSessionId string, resp map[string]interface{}) {
	sessions := ws.SessionList.GetSessionsById(sSessionId)
	if sessions == nil {
		log.Error("websocket sessionId Not Exist: " + sSessionId)
		return
	}
	for _, session := range sessions {
		ws.respondToSession(session, resp)
	}
}

func (ws *WsServer) PushTxResult(txHashStr string, resp map[string]interface{}) {
	ws.Lock()
	defer ws.Unlock()
	sSessionId := ws.TxHashMap[txHashStr]
	delete(ws.TxHashMap, txHashStr)
	if len(sSessionId) > 0 {
		ws.respondToId(sSessionId, resp)
	}
	ws.PushResult(resp)
}

func (ws *WsServer) PushResult(resp map[string]interface{}) {
	resp["Desc"] = common.ErrMessage[resp["Error"].(common.ErrCode)]
	data, err := json.Marshal(resp)
	if err != nil {
		log.Error("Websocket PushResult:", err)
		return
	}
	ws.Broadcast(data)
}

func (ws *WsServer) Broadcast(data []byte) error {
	ws.SessionList.ForEachSession(func(s *session.Session) {
		s.SendText(data)
	})
	return nil
}

func (ws *WsServer) initTlsListen() (net.Listener, error) {

	CertPath := config.Parameters.RestCertPath
	KeyPath := config.Parameters.RestKeyPath

	// load cert
	cert, err := tls.LoadX509KeyPair(CertPath, KeyPath)
	if err != nil {
		log.Error("load keys fail", err)
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	log.Info("TLS listen port is ", strconv.Itoa(int(config.Parameters.HttpWsPort)))
	listener, err := tls.Listen("tcp", ":"+strconv.Itoa(int(config.Parameters.HttpWsPort)), tlsConfig)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return listener, nil
}

func (ws *WsServer) GetClientsById(cliendID []byte) []*session.Session {
	sessions := ws.SessionList.GetSessionsById(hex.EncodeToString(cliendID))
	return sessions
}

func (ws *WsServer) GetNetNode() (*node.LocalNode, error) {
	return ws.localNode, nil
}

func (ws *WsServer) GetWallet() (vault.Wallet, error) {
	return ws.wallet, nil
}

func (ws *WsServer) NotifyWrongClients() {
	ws.SessionList.ForEachClient(func(client *session.Session) {
		clientID := client.GetID()
		if clientID == nil {
			return
		}

		localNode, err := ws.GetNetNode()
		if err != nil {
			return
		}

		addr, pubkey, id, err := localNode.FindWsAddr(clientID)
		if err != nil {
			log.Errorf("Find websocket address error: %v", err)
			return
		}

		if addr != localNode.GetWsAddr() {
			resp := common.ResponsePack(common.WRONG_NODE)
			resp["Result"] = common.NodeInfo(addr, pubkey, id)
			ws.respondToSession(client, resp)
		}
	})
}

func (ws *WsServer) sendInboundRelayMessageToClient(v interface{}) {
	if msg, ok := v.(*pb.Relay); ok {
		ws.sendInboundRelayMessage(msg)
	} else {
		log.Error("Decode relay message failed")
	}
}
