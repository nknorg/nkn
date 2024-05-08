package server

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	api "github.com/nknorg/nkn/v2/api/common"
	"github.com/nknorg/nkn/v2/api/common/errcode"
	"github.com/nknorg/nkn/v2/api/webrtc"
	"github.com/nknorg/nkn/v2/api/websocket/messagebuffer"
	"github.com/nknorg/nkn/v2/api/websocket/session"
	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/crypto"
	"github.com/nknorg/nkn/v2/event"
	"github.com/nknorg/nkn/v2/node"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/util/address"
	"github.com/nknorg/nkn/v2/util/log"
	"github.com/nknorg/nkn/v2/vault"

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
	checkWrongClientsInterval    = time.Minute
)

type Handler struct {
	handler  api.Handler
	pushFlag bool
}

type MsgServer struct {
	sync.RWMutex
	SessionList           *session.SessionList
	ActionMap             map[string]Handler
	TxHashMap             map[string]string //key: txHash   value:sessionid
	localNode             node.ILocalNode
	wallet                *vault.Wallet
	messageBuffer         *messagebuffer.MessageBuffer
	messageDeliveredCache *DelayedChan
	sigChainCache         common.Cache
	ws                    *wsServer
}

func InitMsgServer(localNode node.ILocalNode, wallet *vault.Wallet) *MsgServer {
	ws := &MsgServer{
		SessionList:           session.NewSessionList(),
		TxHashMap:             make(map[string]string),
		localNode:             localNode,
		wallet:                wallet,
		messageBuffer:         messagebuffer.NewMessageBuffer(true),
		messageDeliveredCache: NewDelayedChan(messageDeliveredCacheSize, pongTimeout),
		sigChainCache:         common.NewGoCache(sigChainCacheExpiration, sigChainCacheCleanupInterval),
		ws:                    &wsServer{},
	}
	return ws
}

func (ms *MsgServer) Start(wssCertReady chan struct{}) error {

	ms.ws.Start(ms, wssCertReady)
	ms.registryMethod()

	go ms.startCheckingLostMessages()
	go ms.startCheckingWrongClients()

	event.Queue.Subscribe(event.SendInboundMessageToClient, ms.sendInboundRelayMessageToClient)

	webrtc.NewConnection = ms.newConnection

	return nil
}

func (ms *MsgServer) registryMethod() {
	gettxhashmap := func(s api.Serverer, cmd map[string]interface{}, ctx context.Context) map[string]interface{} {
		ms.Lock()
		defer ms.Unlock()
		resp := api.RespPacking(len(ms.TxHashMap), errcode.SUCCESS)
		return resp
	}

	heartbeat := func(s api.Serverer, cmd map[string]interface{}, ctx context.Context) map[string]interface{} {
		return api.RespPacking(cmd["Userid"], errcode.SUCCESS)

	}

	getsessioncount := func(s api.Serverer, cmd map[string]interface{}, ctx context.Context) map[string]interface{} {
		return api.RespPacking(ms.SessionList.GetSessionCount(), errcode.SUCCESS)
	}

	setClient := func(s api.Serverer, cmd map[string]interface{}, ctx context.Context) map[string]interface{} {
		addrStr, ok := cmd["Addr"].(string)
		if !ok {
			return api.RespPacking(nil, errcode.INVALID_PARAMS)
		}

		clientID, pubKey, _, err := address.ParseClientAddress(addrStr)
		if err != nil {
			log.Error("Parse client address error:", err)
			return api.RespPacking(nil, errcode.INVALID_PARAMS)
		}

		err = crypto.CheckPublicKey(pubKey)
		if err != nil {
			log.Error("Invalid public key hex:", err)
			return api.RespPacking(nil, errcode.INVALID_PARAMS)
		}

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
			return api.RespPacking(nil, errcode.INTERNAL_ERROR)
		}

		if wsAddr != localAddr {
			return api.RespPacking(api.NodeInfo(wsAddr, rpcAddr, pubkey, id, ""), errcode.WRONG_NODE)
		}

		// client auth
		signature, okSig := cmd["Signature"]
		clientSalt, okSalt := cmd["ClientSalt"]

		if okSig && okSalt { // if client send ClientSalt and Signature, then check it
			strSignature, typeOk := signature.(string) // interface type assertion
			if !typeOk {
				return api.RespPacking(errors.New("invalid signature"), errcode.INVALID_PARAMS)
			}
			byteSignature, err := hex.DecodeString(strSignature)
			if err != nil {
				return api.RespPacking(err.Error(), errcode.ILLEGAL_DATAFORMAT)
			}

			strClientSalt, typeOk := clientSalt.(string) // interface type assertion
			if !typeOk {
				return api.RespPacking(errors.New("invalid salt"), errcode.INVALID_PARAMS)
			}
			byteClientSalt, err := hex.DecodeString(strClientSalt)
			if err != nil {
				return api.RespPacking(err.Error(), errcode.ILLEGAL_DATAFORMAT)
			}

			sess := cmd["session"].(*session.Session)
			challenge := sess.Challenge[:]
			challenge = append(challenge, byteClientSalt...)
			hash := sha256.Sum256(challenge)

			err = crypto.Verify(pubKey, hash[:], byteSignature)
			if err != nil { // fail verify challenge signature
				go func() {
					log.Warning("Client signature is not right, close its conneciton now")
					time.Sleep(3 * time.Second)       // sleep several second, let response reach client
					ms.SessionList.CloseSession(sess) // close this session
				}()
				return api.RespPacking(nil, errcode.INVALID_SIGNATURE)
			}
		} else {
			log.Debugf("client didn't send signature")
		}

		newSessionID := hex.EncodeToString(clientID)
		session, err := ms.SessionList.ChangeSessionToClient(cmd["Userid"].(string), newSessionID)
		if err != nil {
			log.Error("Change session id error: ", err)
			return api.RespPacking(nil, errcode.INTERNAL_ERROR)
		}
		session.SetClient(clientID, pubKey, &addrStr, isTlsClient)

		go func() {
			messages := ms.messageBuffer.PopMessages(clientID)
			for _, message := range messages {
				ms.sendInboundRelayMessage(message, true)
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
		res["node"] = api.NodeInfo(wsAddr, rpcAddr, pubkey, id, "")
		res["sigChainBlockHash"] = hex.EncodeToString(sigChainBlockHash.ToArray())

		return api.RespPacking(res, errcode.SUCCESS)
	}

	actionMap := map[string]Handler{
		"heartbeat":       {handler: heartbeat},
		"gettxhashmap":    {handler: gettxhashmap},
		"getsessioncount": {handler: getsessioncount},
		"setClient":       {handler: setClient},
	}

	for name, handler := range api.InitialAPIHandlers {
		if handler.IsAccessableByWebsocket() {
			actionMap[name] = Handler{handler: handler.Handler}
		}
	}

	ms.ActionMap = actionMap
}

func (ms *MsgServer) IsValidMsg(reqMsg map[string]interface{}) bool {
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

func (ms *MsgServer) OnDataHandle(curSession *session.Session, messageType int, bysMsg []byte, httpr *http.Request) error {
	if messageType == websocket.BinaryMessage {
		msg := &pb.ClientMessage{}
		err := proto.Unmarshal(bysMsg, msg)
		if err != nil {
			return fmt.Errorf("Parse client message error: %v", err)
		}

		var r io.Reader = bytes.NewReader(msg.Message)
		switch msg.CompressionType {
		case pb.CompressionType_COMPRESSION_NONE:
		case pb.CompressionType_COMPRESSION_ZLIB:
			r, err = zlib.NewReader(r)
			if err != nil {
				return fmt.Errorf("Create zlib reader error: %v", err)
			}
			defer r.(io.ReadCloser).Close()
		default:
			return fmt.Errorf("Unsupported message compression type %v", msg.CompressionType)
		}

		b, err := io.ReadAll(io.LimitReader(r, config.MaxClientMessageSize+1))
		if err != nil {
			return fmt.Errorf("ReadAll from reader error: %v", err)
		}
		if len(b) > config.MaxClientMessageSize {
			return fmt.Errorf("Max client message size reached.")
		}

		switch msg.MessageType {
		case pb.ClientMessageType_OUTBOUND_MESSAGE:
			outboundMsg := &pb.OutboundMessage{}
			err = proto.Unmarshal(b, outboundMsg)
			if err != nil {
				return fmt.Errorf("Unmarshal outbound message error: %v", err)
			}
			ms.sendOutboundRelayMessage(curSession.GetAddrStr(), outboundMsg)
		case pb.ClientMessageType_RECEIPT:
			receipt := &pb.Receipt{}
			err = proto.Unmarshal(b, receipt)
			if err != nil {
				return fmt.Errorf("Unmarshal receipt error: %v", err)
			}
			err = ms.handleReceipt(receipt)
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
		resp := api.ResponsePack(errcode.ILLEGAL_DATAFORMAT)
		ms.respondToSession(curSession, resp)
		return fmt.Errorf("websocket OnDataHandle: %v", err)
	}
	actionName, ok := req["Action"].(string)
	if !ok {
		resp := api.ResponsePack(errcode.INVALID_METHOD)
		ms.respondToSession(curSession, resp)
		return nil
	}
	action, ok := ms.ActionMap[actionName]
	if !ok {
		resp := api.ResponsePack(errcode.INVALID_METHOD)
		ms.respondToSession(curSession, resp)
		return nil
	}
	if !ms.IsValidMsg(req) {
		resp := api.ResponsePack(errcode.INVALID_PARAMS)
		ms.respondToSession(curSession, resp)
		return nil
	}
	if height, ok := req["Height"].(float64); ok {
		req["Height"] = strconv.FormatInt(int64(height), 10)
	}
	if raw, ok := req["Raw"].(float64); ok {
		req["Raw"] = strconv.FormatInt(int64(raw), 10)
	}
	req["Userid"] = curSession.GetSessionId()
	ctx := context.Background()
	if httpr != nil {
		req["IsTls"] = httpr.TLS != nil
		ctx = httpr.Context()
	} else {
		req["IsTls"] = false
	}
	req["session"] = curSession
	ret := action.handler(ms, req, ctx)
	resp := api.ResponsePack(ret["error"].(errcode.ErrCode))
	resp["Action"] = actionName
	resp["Result"] = ret["resultOrData"]
	if txHash, ok := resp["Result"].(string); ok && action.pushFlag {
		ms.Lock()
		defer ms.Unlock()
		ms.TxHashMap[txHash] = curSession.GetSessionId()
	}
	ms.respondToSession(curSession, resp)

	return nil
}

func (ms *MsgServer) SetTxHashMap(txhash string, sessionid string) {
	ms.Lock()
	defer ms.Unlock()
	ms.TxHashMap[txhash] = sessionid
}

func (ms *MsgServer) deleteTxHashs(sSessionId string) {
	ms.Lock()
	defer ms.Unlock()
	for k, v := range ms.TxHashMap {
		if v == sSessionId {
			delete(ms.TxHashMap, k)
		}
	}
}

func (ms *MsgServer) respondToSession(session *session.Session, resp map[string]interface{}) error {
	resp["Desc"] = errcode.ErrMessage[resp["Error"].(errcode.ErrCode)]
	data, err := json.Marshal(resp)
	if err != nil {
		log.Error("Websocket response:", err)
		return err
	}
	err = session.SendText(data)
	return err
}

func (ms *MsgServer) respondToId(sSessionId string, resp map[string]interface{}) {
	sessions := ms.SessionList.GetSessionsById(sSessionId)
	if sessions == nil {
		log.Error("websocket sessionId Not Exist: " + sSessionId)
		return
	}
	for _, session := range sessions {
		ms.respondToSession(session, resp)
	}
}

func (ms *MsgServer) PushTxResult(txHashStr string, resp map[string]interface{}) {
	ms.Lock()
	defer ms.Unlock()
	sSessionId := ms.TxHashMap[txHashStr]
	delete(ms.TxHashMap, txHashStr)
	if len(sSessionId) > 0 {
		ms.respondToId(sSessionId, resp)
	}
	ms.PushResult(resp)
}

func (ms *MsgServer) PushResult(resp map[string]interface{}) {
	resp["Desc"] = errcode.ErrMessage[resp["Error"].(errcode.ErrCode)]
	data, err := json.Marshal(resp)
	if err != nil {
		log.Error("Websocket PushResult:", err)
		return
	}
	ms.Broadcast(data)
}

func (ms *MsgServer) Broadcast(data []byte) error {
	ms.SessionList.ForEachSession(func(s *session.Session) {
		s.SendText(data)
	})
	return nil
}

func (ms *MsgServer) initTlsListen() (net.Listener, error) {
	tlsConfig := &tls.Config{
		GetCertificate: api.GetWssCertificate,
	}

	listener, err := tls.Listen("tcp", ":"+strconv.Itoa(int(config.Parameters.HttpWssPort)), tlsConfig)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return listener, nil
}

func (ms *MsgServer) GetClientsById(cliendID []byte) []*session.Session {
	sessions := ms.SessionList.GetSessionsById(hex.EncodeToString(cliendID))
	return sessions
}

func (ms *MsgServer) GetNetNode() node.ILocalNode {
	return ms.localNode
}

func (ms *MsgServer) NotifyWrongClients() {
	ms.SessionList.ForEachClient(func(client *session.Session) {
		clientID := client.GetID()
		if clientID == nil {
			return
		}

		localNode := ms.GetNetNode()

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
			resp := api.ResponsePack(errcode.WRONG_NODE)
			resp["Result"] = api.NodeInfo(wsAddr, rpcAddr, pubkey, id, "")
			ms.respondToSession(client, resp)
		}
	})
}

func (ms *MsgServer) startCheckingWrongClients() {
	for {
		time.Sleep(checkWrongClientsInterval)
		ms.NotifyWrongClients()
	}
}

func (ms *MsgServer) sendInboundRelayMessageToClient(v interface{}) {
	if msg, ok := v.(*pb.Relay); ok {
		ms.sendInboundRelayMessage(msg, true)
	} else {
		log.Error("Decode relay message failed")
	}
}

// client auth, generate challenge
func (ms *MsgServer) sendClientAuthChallenge(sess *session.Session) error {
	resp := api.ResponsePack(errcode.SUCCESS)
	resp["Action"] = "authChallenge"

	challenge := make([]byte, 32)
	rand.Reader.Read(challenge)
	resp["Challenge"] = hex.EncodeToString(challenge)
	sess.Challenge = challenge // save this challenge for verifying later.

	err := ms.respondToSession(sess, resp)
	return err
}

func (ms *MsgServer) newConnection(conn session.Conn, r *http.Request) {
	sess, err := ms.SessionList.NewSession(conn)
	if err != nil {
		log.Error("websocket NewSession:", err)
		return
	}

	defer func() {
		ms.deleteTxHashs(sess.GetSessionId())
		ms.SessionList.CloseSession(sess)
		if err := recover(); err != nil {
			log.Error("websocket recover:", err)
		}
	}()

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongTimeout))
		sess.UpdateLastReadTime()
		return nil
	})

	// client auth
	err = ms.sendClientAuthChallenge(sess)
	if err != nil {
		log.Error("send client auth challenge: ", err)
		return
	}

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
		messageType, bysMsg, err := conn.ReadMessage()
		if err != nil {
			log.Debugf("websocket read message error: %v", err)
			break
		}
		conn.SetReadDeadline(time.Now().Add(pongTimeout))
		sess.UpdateLastReadTime()

		err = ms.OnDataHandle(sess, messageType, bysMsg, r)
		if err != nil {
			log.Error(err)
		}
	}
}
