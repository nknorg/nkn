package server

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
	TlsPort                      = 443
	sigChainCacheExpiration      = config.ConsensusTimeout
	sigChainCacheCleanupInterval = time.Second
	pingInterval                 = 8 * time.Second
	pongTimeout                  = 10 * time.Second // should be greater than pingInterval
	maxMessageSize               = config.MaxClientMessageSize
	messageDeliveredCacheSize    = 65536
)

type Handler struct {
	handler  common.Handler
	pushFlag bool
}

type WsServer struct {
	sync.RWMutex
	Upgrader              websocket.Upgrader
	listener              net.Listener
	tlsListener           net.Listener
	server                *http.Server
	tlsServer             *http.Server
	SessionList           *session.SessionList
	ActionMap             map[string]Handler
	TxHashMap             map[string]string //key: txHash   value:sessionid
	localNode             *node.LocalNode
	wallet                *vault.Wallet
	messageBuffer         *messagebuffer.MessageBuffer
	messageDeliveredCache *DelayedChan
	sigChainCache         Cache
}

func InitWsServer(localNode *node.LocalNode, wallet *vault.Wallet) *WsServer {
	ws := &WsServer{
		Upgrader:              websocket.Upgrader{},
		SessionList:           session.NewSessionList(),
		TxHashMap:             make(map[string]string),
		localNode:             localNode,
		wallet:                wallet,
		messageBuffer:         messagebuffer.NewMessageBuffer(),
		messageDeliveredCache: NewDelayedChan(messageDeliveredCacheSize, pongTimeout),
		sigChainCache:         NewGoCache(sigChainCacheExpiration, sigChainCacheCleanupInterval),
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

	var err error
	ws.tlsListener, err = ws.initTlsListen()
	if err != nil {
		log.Error("Https Cert: ", err.Error())
		return err
	}

	ws.listener, err = net.Listen("tcp", ":"+strconv.Itoa(int(config.Parameters.HttpWsPort)))
	if err != nil {
		log.Error("net.Listen: ", err.Error())
		return err
	}

	event.Queue.Subscribe(event.SendInboundMessageToClient, ws.sendInboundRelayMessageToClient)

	ws.server = &http.Server{Handler: http.HandlerFunc(ws.websocketHandler)}
	go ws.server.Serve(ws.listener)

	ws.tlsServer = &http.Server{Handler: http.HandlerFunc(ws.websocketHandler)}
	go ws.tlsServer.Serve(ws.tlsListener)

	go ws.startCheckingLostMessages()

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

		err = crypto.CheckPublicKey(pubKey)
		if err != nil {
			log.Error("Invalid public key hex:", err)
			return common.RespPacking(nil, common.INVALID_PARAMS)
		}

		// TODO: use signature (or better, with one-time challenge) to verify identity

		localNode := s.GetNetNode()

		isTlsClient := cmd["IsTls"].(bool)
		var wsAddr, rpcAddr, localAddr string
		var pubkey, id []byte

		if isTlsClient {
			wsAddr, rpcAddr, pubkey, id, err = localNode.FindWssAddr(clientID)
			localAddr = localNode.GetWssAddr()
		} else {
			wsAddr, rpcAddr, pubkey, id, err = localNode.FindWsAddr(clientID)
			localAddr = localNode.GetWsAddr()
		}
		if err != nil {
			log.Errorf("Find websocket address error: %v", err)
			return common.RespPacking(nil, common.INTERNAL_ERROR)
		}

		if wsAddr != localAddr {
			return common.RespPacking(common.NodeInfo(wsAddr, rpcAddr, pubkey, id), common.WRONG_NODE)
		}

		newSessionID := hex.EncodeToString(clientID)
		session, err := ws.SessionList.ChangeSessionToClient(cmd["Userid"].(string), newSessionID)
		if err != nil {
			log.Error("Change session id error: ", err)
			return common.RespPacking(nil, common.INTERNAL_ERROR)
		}
		session.SetClient(clientID, pubKey, &addrStr, isTlsClient)

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
		res["node"] = common.NodeInfo(wsAddr, rpcAddr, pubkey, id)
		res["sigChainBlockHash"] = hex.EncodeToString(sigChainBlockHash.ToArray())

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

//websocketHandler
func (ws *WsServer) websocketHandler(w http.ResponseWriter, r *http.Request) {
	wsConn, err := ws.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("websocket Upgrader: ", err)
		return
	}
	defer wsConn.Close()

	sess, err := ws.SessionList.NewSession(wsConn)
	if err != nil {
		log.Error("websocket NewSession:", err)
		return
	}

	defer func() {
		ws.deleteTxHashs(sess.GetSessionId())
		ws.SessionList.CloseSession(sess)
		if err := recover(); err != nil {
			log.Error("websocket recover:", err)
		}
	}()

	wsConn.SetReadLimit(maxMessageSize)
	wsConn.SetReadDeadline(time.Now().Add(pongTimeout))
	wsConn.SetPongHandler(func(string) error {
		wsConn.SetReadDeadline(time.Now().Add(pongTimeout))
		sess.UpdateLastReadTime()
		return nil
	})

	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		var err error
		for {
			select {
			case <-ticker.C:
				err = sess.Ping()
				if err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	for {
		messageType, bysMsg, err := wsConn.ReadMessage()
		if err != nil {
			log.Debugf("websocket read message error: %v", err)
			break
		}

		wsConn.SetReadDeadline(time.Now().Add(pongTimeout))
		sess.UpdateLastReadTime()

		err = ws.OnDataHandle(sess, messageType, bysMsg, r)
		if err != nil {
			log.Error(err)
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

func (ws *WsServer) OnDataHandle(curSession *session.Session, messageType int, bysMsg []byte, r *http.Request) error {
	if messageType == websocket.BinaryMessage {
		msg := &pb.ClientMessage{}
		err := proto.Unmarshal(bysMsg, msg)
		if err != nil {
			return fmt.Errorf("Parse client message error: %v", err)
		}

		var r io.Reader = bytes.NewReader(msg.Message)
		switch msg.CompressionType {
		case pb.COMPRESSION_NONE:
		case pb.COMPRESSION_ZLIB:
			r, err = zlib.NewReader(r)
			if err != nil {
				return fmt.Errorf("Create zlib reader error: %v", err)
			}
			defer r.(io.ReadCloser).Close()
		default:
			return fmt.Errorf("Unsupported message compression type %v", msg.CompressionType)
		}

		b, err := ioutil.ReadAll(io.LimitReader(r, config.MaxClientMessageSize+1))
		if err != nil {
			return fmt.Errorf("ReadAll from reader error: %v", err)
		}
		if len(b) > config.MaxClientMessageSize {
			return fmt.Errorf("Max client message size reached.")
		}

		switch msg.MessageType {
		case pb.OUTBOUND_MESSAGE:
			outboundMsg := &pb.OutboundMessage{}
			err = proto.Unmarshal(b, outboundMsg)
			if err != nil {
				return fmt.Errorf("Unmarshal outbound message error: %v", err)
			}
			ws.sendOutboundRelayMessage(curSession.GetAddrStr(), outboundMsg)
		case pb.RECEIPT:
			receipt := &pb.Receipt{}
			err = proto.Unmarshal(b, receipt)
			if err != nil {
				return fmt.Errorf("Unmarshal receipt error: %v", err)
			}
			err = ws.handleReceipt(receipt)
			if err != nil {
				return fmt.Errorf("Handle receipt error: %v", err)
			}
		default:
			return fmt.Errorf("unsupported client message type %v", msg.MessageType)
		}

		return nil
	}

	var req = make(map[string]interface{})

	if err := json.Unmarshal(bysMsg, &req); err != nil {
		resp := common.ResponsePack(common.ILLEGAL_DATAFORMAT)
		ws.respondToSession(curSession, resp)
		return fmt.Errorf("websocket OnDataHandle: %v", err)
	}
	actionName, ok := req["Action"].(string)
	if !ok {
		resp := common.ResponsePack(common.INVALID_METHOD)
		ws.respondToSession(curSession, resp)
		return nil
	}
	action, ok := ws.ActionMap[actionName]
	if !ok {
		resp := common.ResponsePack(common.INVALID_METHOD)
		ws.respondToSession(curSession, resp)
		return nil
	}
	if !ws.IsValidMsg(req) {
		resp := common.ResponsePack(common.INVALID_PARAMS)
		ws.respondToSession(curSession, resp)
		return nil
	}
	if height, ok := req["Height"].(float64); ok {
		req["Height"] = strconv.FormatInt(int64(height), 10)
	}
	if raw, ok := req["Raw"].(float64); ok {
		req["Raw"] = strconv.FormatInt(int64(raw), 10)
	}
	req["Userid"] = curSession.GetSessionId()
	req["IsTls"] = r.TLS != nil
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

	return nil
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

	CertPath := config.Parameters.HttpWssCert
	KeyPath := config.Parameters.HttpWssKey

	// load cert
	cert, err := tls.LoadX509KeyPair(CertPath, KeyPath)
	if err != nil {
		log.Error("load keys fail", err)
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	listener, err := tls.Listen("tcp", ":"+strconv.Itoa(int(config.Parameters.HttpWssPort)), tlsConfig)
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

func (ws *WsServer) GetNetNode() *node.LocalNode {
	return ws.localNode
}

func (ws *WsServer) NotifyWrongClients() {
	ws.SessionList.ForEachClient(func(client *session.Session) {
		clientID := client.GetID()
		if clientID == nil {
			return
		}

		localNode := ws.GetNetNode()

		var wsAddr, rpcAddr, localAddr string
		var pubkey, id []byte
		var err error

		if client.IsTlsClient() {
			wsAddr, rpcAddr, pubkey, id, err = localNode.FindWssAddr(clientID)
			localAddr = localNode.GetWssAddr()
		} else {
			wsAddr, rpcAddr, pubkey, id, err = localNode.FindWsAddr(clientID)
			localAddr = localNode.GetWsAddr()
		}
		if err != nil {
			log.Errorf("Find websocket address error: %v", err)
			return
		}

		if wsAddr != localAddr {
			resp := common.ResponsePack(common.WRONG_NODE)
			resp["Result"] = common.NodeInfo(wsAddr, rpcAddr, pubkey, id)
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
