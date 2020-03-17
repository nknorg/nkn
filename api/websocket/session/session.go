package session

import (
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pborman/uuid"
)

const (
	writeTimeout = 10 * time.Second
)

type Session struct {
	ws        *websocket.Conn
	sessionID string

	sync.RWMutex
	clientChordID []byte
	clientPubKey  []byte
	clientAddrStr *string
	isTlsClient   bool
}

func (s *Session) GetSessionId() string {
	return s.sessionID
}

func newSession(wsConn *websocket.Conn) (session *Session, err error) {
	sessionID := uuid.NewUUID().String()
	session = &Session{
		ws:        wsConn,
		sessionID: sessionID,
	}
	return session, nil
}

func (s *Session) close() {
	s.Lock()
	defer s.Unlock()
	if s.ws != nil {
		s.ws.Close()
		s.ws = nil
	}
	s.sessionID = ""
}

func (s *Session) Send(msgType int, data []byte) error {
	s.RLock()
	defer s.RUnlock()
	if s.ws == nil {
		return errors.New("Websocket is null")
	}
	s.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
	return s.ws.WriteMessage(msgType, data)
}

func (s *Session) SendText(data []byte) error {
	return s.Send(websocket.TextMessage, data)
}

func (s *Session) SendBinary(data []byte) error {
	return s.Send(websocket.BinaryMessage, data)
}

func (s *Session) Ping() error {
	return s.Send(websocket.PingMessage, nil)
}

func (s *Session) SetSessionId(sessionId string) {
	s.Lock()
	defer s.Unlock()
	s.sessionID = sessionId
}

func (s *Session) SetClient(chordID, pubKey []byte, addrStr *string, isTls bool) {
	s.Lock()
	defer s.Unlock()
	s.clientChordID = chordID
	s.clientPubKey = pubKey
	s.clientAddrStr = addrStr
	s.isTlsClient = isTls
}

func (s *Session) isClient() bool {
	return s.clientChordID != nil && s.clientPubKey != nil && s.clientAddrStr != nil
}

func (s *Session) IsClient() bool {
	s.RLock()
	defer s.RUnlock()
	return s.isClient()
}

func (s *Session) GetID() []byte {
	s.RLock()
	defer s.RUnlock()
	if !s.isClient() {
		return nil
	}
	return s.clientChordID
}

func (s *Session) GetPubKey() []byte {
	s.RLock()
	defer s.RUnlock()
	if !s.isClient() {
		return nil
	}
	return s.clientPubKey
}

func (s *Session) GetAddrStr() *string {
	s.RLock()
	defer s.RUnlock()
	if !s.isClient() {
		return nil
	}
	return s.clientAddrStr
}

func (s *Session) IsTlsClient() bool {
	s.RLock()
	defer s.RUnlock()
	return s.isTlsClient
}
