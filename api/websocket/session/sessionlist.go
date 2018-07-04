package session

import (
	"errors"
	"sync"

	"github.com/gorilla/websocket"
)

type SessionList struct {
	sync.RWMutex
	mapOnlineList map[string][]*Session //key is SessionId
}

func NewSessionList() *SessionList {
	return &SessionList{
		mapOnlineList: make(map[string][]*Session),
	}
}

func (sl *SessionList) NewSession(wsConn *websocket.Conn) (*Session, error) {
	session, err := newSession(wsConn)
	if err != nil {
		return nil, err
	}
	err = sl.addOnlineSession(session)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (sl *SessionList) CloseSession(session *Session) error {
	if session == nil {
		return errors.New("Session is nil")
	}
	err := sl.removeSession(session)
	if err != nil {
		return err
	}
	session.close()
	return nil
}

func (sl *SessionList) addOnlineSession(session *Session) error {
	sessionId := session.GetSessionId()
	if sessionId == "" {
		return errors.New("Session id is empty")
	}
	sl.Lock()
	defer sl.Unlock()
	sl.mapOnlineList[sessionId] = append(sl.mapOnlineList[sessionId], session)
	return nil
}

func (sl *SessionList) removeSession(session *Session) error {
	sl.Lock()
	defer sl.Unlock()
	sessionId := session.GetSessionId()
	sessions := sl.mapOnlineList[sessionId]
	for i, s := range sessions {
		if s == session {
			sl.mapOnlineList[sessionId] = append(sessions[:i], sessions[i+1:]...)
			if len(sl.mapOnlineList[sessionId]) == 0 {
				delete(sl.mapOnlineList, sessionId)
			}
			return nil
		}
	}
	return errors.New("Session not found")
}

func (sl *SessionList) GetSessionsById(sSessionId string) []*Session {
	sl.RLock()
	defer sl.RUnlock()
	sessions := sl.mapOnlineList[sSessionId]
	if len(sessions) > 0 {
		return sessions
	}
	return nil
}

func (sl *SessionList) GetSessionCount() int {
	var count int
	sl.RLock()
	defer sl.RUnlock()
	for _, sessions := range sl.mapOnlineList {
		count += len(sessions)
	}
	return count
}

func (sl *SessionList) ForEachSession(visit func(*Session)) {
	sl.RLock()
	defer sl.RUnlock()
	for _, sessions := range sl.mapOnlineList {
		for _, session := range sessions {
			visit(session)
		}
	}
}

func (sl *SessionList) ForEachClient(visit func(*Session)) {
	sl.ForEachSession(func(session *Session) {
		if session.IsClient() {
			visit(session)
		}
	})
}

func (sl *SessionList) ChangeSessionToClient(sessionId, clientId string) (*Session, error) {
	sessions := sl.GetSessionsById(sessionId)
	if len(sessions) == 0 {
		return nil, errors.New("Session not exists")
	}
	if len(sessions) > 1 {
		return nil, errors.New("More than one session exists")
	}
	session := sessions[0]

	err := sl.removeSession(session)
	if err != nil {
		return nil, err
	}

	session.SetSessionId(clientId)

	err = sl.addOnlineSession(session)
	if err != nil {
		return nil, err
	}

	return session, nil
}
