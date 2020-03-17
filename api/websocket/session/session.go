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
	sync.Mutex
	ws            *websocket.Conn
	sSessionId    string
	clientChordID []byte
	clientPubKey  []byte
	clientAddrStr *string
	isTlsClient   bool
}

func (s *Session) GetSessionId() string {
	return s.sSessionId
}

func newSession(wsConn *websocket.Conn) (session *Session, err error) {
	sSessionId := uuid.NewUUID().String()
	session = &Session{
		ws:         wsConn,
		sSessionId: sSessionId,
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
	s.sSessionId = ""
}

func (s *Session) Send(msgType int, data []byte) error {
	s.Lock()
	defer s.Unlock()
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
	s.sSessionId = sessionId
}

func (s *Session) SetClient(chordID, pubKey []byte, addrStr *string, isTls bool) {
	s.Lock()
	defer s.Unlock()
	s.clientChordID = chordID
	s.clientPubKey = pubKey
	s.clientAddrStr = addrStr
	s.isTlsClient = isTls
}

func (s *Session) IsClient() bool {
	return s.clientChordID != nil && s.clientPubKey != nil && s.clientAddrStr != nil
}

func (s *Session) GetID() []byte {
	if !s.IsClient() {
		return nil
	}
	return s.clientChordID
}

func (s *Session) GetPubKey() []byte {
	if !s.IsClient() {
		return nil
	}
	return s.clientPubKey
}

func (s *Session) GetAddrStr() *string {
	if !s.IsClient() {
		return nil
	}
	return s.clientAddrStr
}

func (s *Session) IsTlsClient() bool {
	return s.isTlsClient
}
