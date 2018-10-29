package transport

import (
	"fmt"
	"net"

	kcp "github.com/xtaci/kcp-go"
)

// KCPTransport is the transport layer based on KCP protocol
type KCPTransport struct{}

// NewKCPTransport creates a new KCP transport layer
func NewKCPTransport() *KCPTransport {
	t := &KCPTransport{}
	return t
}

// Dial connects to the remote address on the network "udp"
func (t *KCPTransport) Dial(addr string) (net.Conn, error) {
	return kcp.Dial(addr)
}

// Listen listens for incoming packets to "port" on the network "udp"
func (t *KCPTransport) Listen(port uint16) (net.Listener, error) {
	laddr := fmt.Sprintf(":%d", port)
	return kcp.Listen(laddr)
}

// GetNetwork returns the network used (tcp or udp)
func (t *KCPTransport) GetNetwork() string {
	return "udp"
}

func (t *KCPTransport) String() string {
	return "kcp"
}
