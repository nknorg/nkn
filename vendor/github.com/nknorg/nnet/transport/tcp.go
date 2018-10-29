package transport

import (
	"fmt"
	"net"
	"time"
)

// TCPTransport is the transport layer based on TCP protocol
type TCPTransport struct {
	dialTimeout time.Duration
}

// NewTCPTransport creates a new TCP transport layer
func NewTCPTransport(dialTimeout time.Duration) *TCPTransport {
	t := &TCPTransport{
		dialTimeout: dialTimeout,
	}
	return t
}

// Dial connects to the remote address on the network "tcp"
func (t *TCPTransport) Dial(addr string) (net.Conn, error) {
	return net.DialTimeout(t.GetNetwork(), addr, t.dialTimeout)
}

// Listen listens for incoming packets to "port" on the network "tcp"
func (t *TCPTransport) Listen(port uint16) (net.Listener, error) {
	laddr := fmt.Sprintf(":%d", port)
	return net.Listen(t.GetNetwork(), laddr)
}

// GetNetwork returns the network used (tcp or udp)
func (t *TCPTransport) GetNetwork() string {
	return "tcp"
}

func (t *TCPTransport) String() string {
	return t.GetNetwork()
}
