package transport

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
)

// Address is a URI for a node
type Address struct {
	Transport Transport
	Host      string
	Port      uint16
}

// NewAddress creates an Address struct with given protocol and address
func NewAddress(protocol, host string, port uint16) (*Address, error) {
	transport, err := NewTransport(protocol)
	if err != nil {
		return nil, err
	}

	addr := &Address{
		Transport: transport,
		Host:      host,
		Port:      port,
	}

	return addr, nil
}

// Parse parses a raw addr string into an Address struct
func Parse(rawAddr string) (*Address, error) {
	u, err := url.Parse(rawAddr)
	if err != nil {
		return nil, err
	}

	transport, err := NewTransport(u.Scheme)
	if err != nil {
		return nil, err
	}

	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	addr := &Address{
		Transport: transport,
		Host:      host,
		Port:      uint16(port),
	}

	return addr, nil
}

func (addr *Address) String() string {
	return fmt.Sprintf("%s://%s:%d", addr.Transport, addr.Host, addr.Port)
}

// Dial dials the remote address using local transport
func (addr *Address) Dial() (net.Conn, error) {
	return addr.Transport.Dial(fmt.Sprintf("%s:%d", addr.Host, addr.Port))
}
