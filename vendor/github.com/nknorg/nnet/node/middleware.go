package node

import (
	"errors"
)

// BytesReceived is called when local node receive user-defined BYTES message.
// The argument it accepts are bytes data, message ID (can be used to reply
// message), sender ID, and the neighbor that passes you the message (may be
// different from the message sneder). Returns the bytes data to be passed in
// the next middleware and if we should proceed to the next middleware.
type BytesReceived func(data, msgID, srcID []byte, remoteNode *RemoteNode) ([]byte, bool)

// LocalNodeWillStart is called right before local node starts listening and
// handling messages. It can be used to add additional data to local node, etc.
// Returns if we should proceed to the next middleware.
type LocalNodeWillStart func(*LocalNode) bool

// LocalNodeStarted is called right after local node starts listening and
// handling messages. Returns if we should proceed to the next middleware.
type LocalNodeStarted func(*LocalNode) bool

// LocalNodeWillStop is called right before local node stops listening and
// handling messages. Returns if we should proceed to the next middleware.
type LocalNodeWillStop func(*LocalNode) bool

// LocalNodeStopped is called right after local node stops listening and
// handling messages. Returns if we should proceed to the next middleware.
type LocalNodeStopped func(*LocalNode) bool

// RemoteNodeConnected is called when a connection is established with a remote
// node, but the remote node id is typically nil, so it's not a good time to use
// the node yet, but can be used to stop the connection to remote node. Returns
// if we should proceed to the next middleware.
type RemoteNodeConnected func(*RemoteNode) bool

// RemoteNodeReady is called when local node has received the node info from
// remote node and the remote node is ready to use. Returns if we should proceed
// to the next middleware.
type RemoteNodeReady func(*RemoteNode) bool

// RemoteNodeDisconnected is called when connection to remote node is closed.
// The cause of the connection close can be on either local node or remote node.
// Returns if we should proceed to the next middleware.
type RemoteNodeDisconnected func(*RemoteNode) bool

// middlewareStore stores the functions that will be called when certain events
// are triggered or in some pipeline
type middlewareStore struct {
	bytesReceived          []BytesReceived
	localNodeWillStart     []LocalNodeWillStart
	localNodeStarted       []LocalNodeStarted
	localNodeWillStop      []LocalNodeWillStop
	localNodeStopped       []LocalNodeStopped
	remoteNodeConnected    []RemoteNodeConnected
	remoteNodeReady        []RemoteNodeReady
	remoteNodeDisconnected []RemoteNodeDisconnected
}

// newMiddlewareStore creates a middlewareStore
func newMiddlewareStore() *middlewareStore {
	return &middlewareStore{
		bytesReceived:          make([]BytesReceived, 0),
		localNodeWillStart:     make([]LocalNodeWillStart, 0),
		localNodeStarted:       make([]LocalNodeStarted, 0),
		localNodeWillStop:      make([]LocalNodeWillStop, 0),
		localNodeStopped:       make([]LocalNodeStopped, 0),
		remoteNodeConnected:    make([]RemoteNodeConnected, 0),
		remoteNodeReady:        make([]RemoteNodeReady, 0),
		remoteNodeDisconnected: make([]RemoteNodeDisconnected, 0),
	}
}

// ApplyMiddleware add a middleware to the store
func (store *middlewareStore) ApplyMiddleware(f interface{}) error {
	switch f := f.(type) {
	case BytesReceived:
		if f == nil {
			return errors.New("middleware is nil")
		}
		store.bytesReceived = append(store.bytesReceived, f)
	case LocalNodeWillStart:
		if f == nil {
			return errors.New("middleware is nil")
		}
		store.localNodeWillStart = append(store.localNodeWillStart, f)
	case LocalNodeStarted:
		if f == nil {
			return errors.New("middleware is nil")
		}
		store.localNodeStarted = append(store.localNodeStarted, f)
	case LocalNodeWillStop:
		if f == nil {
			return errors.New("middleware is nil")
		}
		store.localNodeWillStop = append(store.localNodeWillStop, f)
	case LocalNodeStopped:
		if f == nil {
			return errors.New("middleware is nil")
		}
		store.localNodeStopped = append(store.localNodeStopped, f)
	case RemoteNodeConnected:
		if f == nil {
			return errors.New("middleware is nil")
		}
		store.remoteNodeConnected = append(store.remoteNodeConnected, f)
	case RemoteNodeReady:
		if f == nil {
			return errors.New("middleware is nil")
		}
		store.remoteNodeReady = append(store.remoteNodeReady, f)
	case RemoteNodeDisconnected:
		if f == nil {
			return errors.New("middleware is nil")
		}
		store.remoteNodeDisconnected = append(store.remoteNodeDisconnected, f)
	default:
		return errors.New("unknown middleware type")
	}

	return nil
}
