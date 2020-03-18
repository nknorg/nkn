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
	sync.RWMutex
	sessionID     string
	clientChordID []byte
	clientPubKey  []byte
	clientAddrStr *string
	isTlsClient   bool
	lastReadTime  time.Time

	wsLock sync.Mutex
	ws     *websocket.Conn
}

func (s *Session) GetSessionId() string {
	return s.sessionID
}

func newSession(wsConn *websocket.Conn) (session *Session, err error) {
	sessionID := uuid.NewUUID().String()
	session = &Session{
		ws:           wsConn,
		sessionID:    sessionID,
		lastReadTime: time.Now(),
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
	s.wsLock.Lock()
	defer s.wsLock.Unlock()
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

func (s *Session) GetLastReadTime() time.Time {
	s.RLock()
	defer s.RUnlock()
	return s.lastReadTime
}

func (s *Session) UpdateLastReadTime() {
	s.Lock()
	s.lastReadTime = time.Now()
	s.Unlock()
}
