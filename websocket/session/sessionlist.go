package session

import (
	"errors"
	"sync"

	"github.com/gorilla/websocket"
)

type SessionList struct {
	sync.RWMutex
	mapOnlineList map[string]*Session //key is SessionId
}

func NewSessionList() *SessionList {
	return &SessionList{
		mapOnlineList: make(map[string]*Session),
	}
}

func (sl *SessionList) NewSession(wsConn *websocket.Conn) (session *Session, err error) {
	session, err = newSession(wsConn)
	if err == nil {
		sl.addOnlineSession(session)
	}
	return session, err
}

func (sl *SessionList) CloseSession(session *Session) {
	if session == nil {
		return
	}
	sl.removeSession(session)
	session.close()
}

func (sl *SessionList) addOnlineSession(session *Session) {
	if session.GetSessionId() == "" {
		return
	}
	sl.Lock()
	defer sl.Unlock()
	sl.mapOnlineList[session.GetSessionId()] = session
}

func (sl *SessionList) removeSession(iSession *Session) (err error) {
	return sl.removeSessionById(iSession.GetSessionId())
}

func (sl *SessionList) removeSessionById(sSessionId string) (err error) {
	if sSessionId == "" {
		return err
	}
	sl.Lock()
	defer sl.Unlock()
	delete(sl.mapOnlineList, sSessionId)
	return nil
}

func (sl *SessionList) GetSessionById(sSessionId string) *Session {
	sl.RLock()
	defer sl.RUnlock()
	if session, ok := sl.mapOnlineList[sSessionId]; ok {
		return session
	}
	return nil
}

func (sl *SessionList) GetSessionCount() int {
	sl.RLock()
	defer sl.RUnlock()
	return len(sl.mapOnlineList)
}

func (sl *SessionList) ForEachSession(visit func(*Session)) {
	sl.RLock()
	defer sl.RUnlock()
	for _, v := range sl.mapOnlineList {
		visit(v)
	}
}

func (sl *SessionList) ChangeSessionId(oldId, newId string) error {
	sl.RLock()
	defer sl.RUnlock()
	if oldId == newId {
		return nil
	}
	if _, ok := sl.mapOnlineList[oldId]; !ok {
		return errors.New("Old session ID not exists")
	}
	if _, ok := sl.mapOnlineList[newId]; ok {
		return errors.New("New session ID already exists")
	}
	sl.mapOnlineList[newId] = sl.mapOnlineList[oldId]
	delete(sl.mapOnlineList, oldId)
	return nil
}
