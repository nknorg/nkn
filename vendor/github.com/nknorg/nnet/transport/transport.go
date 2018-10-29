package transport

import (
	"errors"
	"net"
	"time"
)

const (
	dialTimeout = 5 * time.Second
)

// Transport is an abstract transport layer between local and remote nodes
type Transport interface {
	Dial(addr string) (net.Conn, error)
	Listen(port uint16) (net.Listener, error)
	GetNetwork() string
	String() string
}

// NewTransport creates a transport based on conf
func NewTransport(protocol string) (Transport, error) {
	switch protocol {
	case "kcp":
		return NewKCPTransport(), nil
	case "tcp":
		return NewTCPTransport(dialTimeout), nil
	default:
		return nil, errors.New("Unknown protocol " + protocol)
	}
}
