package session

import (
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pborman/uuid"
)

type Session struct {
	sync.Mutex
	mConnection   *websocket.Conn
	nLastActive   int64
	sSessionId    string
	clientChordID []byte
	clientPubKey  []byte
	clientAddrStr *string
	isTlsClient   bool
}

const sessionTimeOut int64 = 120

func (s *Session) GetSessionId() string {
	return s.sSessionId
}

func newSession(wsConn *websocket.Conn) (session *Session, err error) {
	sSessionId := uuid.NewUUID().String()
	session = &Session{
		mConnection: wsConn,
		nLastActive: time.Now().Unix(),
		sSessionId:  sSessionId,
	}
	return session, nil
}

func (s *Session) close() {
	s.Lock()
	defer s.Unlock()
	if s.mConnection != nil {
		s.mConnection.Close()
		s.mConnection = nil
	}
	s.sSessionId = ""
}

func (s *Session) UpdateActiveTime() {
	s.Lock()
	defer s.Unlock()
	s.nLastActive = time.Now().Unix()
}

func (s *Session) Send(msgType int, data []byte) error {
	s.Lock()
	defer s.Unlock()
	if s.mConnection == nil {
		return errors.New("Websocket is null")
	}
	return s.mConnection.WriteMessage(msgType, data)
}

func (s *Session) SendText(data []byte) error {
	return s.Send(websocket.TextMessage, data)
}

func (s *Session) SendBinary(data []byte) error {
	return s.Send(websocket.BinaryMessage, data)
}

func (s *Session) SessionTimeoverCheck() bool {
	if s.IsClient() {
		return false
	}
	nCurTime := time.Now().Unix()
	if nCurTime-s.nLastActive > sessionTimeOut { //sec
		return true
	}
	return false
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
