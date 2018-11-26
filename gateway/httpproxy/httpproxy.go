package httpproxy

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/xtaci/smux"
)

type HTTPProxy struct {
	// defines a listeners for http proxy, such as "127.0.0.1:30004"
	listener string
	timeout  time.Duration
}

func NewServer() *HTTPProxy {
	return &HTTPProxy{
		listener: ":" + strconv.Itoa(int(config.Parameters.HttpProxyPort)),
		timeout: time.Duration(config.Parameters.HttpProxyDialTimeout),
	}
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

// parseRequestLine parses "GET /foo HTTP/1.1" into its three parts.
func parseRequestLine(line string) (method, requestURI, proto string, ok bool) {
	s1 := strings.Index(line, " ")
	s2 := strings.Index(line[s1+1:], " ")
	if s1 < 0 || s2 < 0 {
		return
	}
	s2 += s1 + 1
	return line[:s1], line[s1+1 : s2], line[s2+1:], true
}

func closeConnection(conn net.Conn) {
	err := conn.Close()
	if err != nil {
		log.Error("Error while closing connection:", err)
	}
}

func (s *HTTPProxy) handleSession(conn net.Conn, session *smux.Session) {
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			log.Error("Couldn't accept stream:", err)
			break
		}

		tp := textproto.NewReader(bufio.NewReader(stream))

		line, err := tp.ReadLine()
		if err != nil {
			log.Error("Couldn't read line:", err)
			closeConnection(stream)
			continue
		}

		method, host, _, _ := parseRequestLine(line)

		// won't proxy HTTP due to security reasons
		if method != http.MethodConnect {
			log.Error("Only CONNECT HTTP method supported")
			stream.Write([]byte("HTTP/1.1 403 Forbidden\r\n\r\n"))
			closeConnection(stream)
			continue
		}

		destConn, err := net.DialTimeout("tcp", host, s.timeout * time.Second)
		if err != nil {
			log.Error("Couldn't connect to host", host, "with error:", err)
			closeConnection(stream)
			continue
		}

		stream.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))

		go transfer(destConn, stream)
		go transfer(stream, destConn)
	}

	session.Close()
	conn.Close()
}

func (s *HTTPProxy) Start() {
	listener, err := net.Listen("tcp", s.listener)
	if err != nil {
		log.Error("Couldn't bind HTTP proxy port:", err)
	}

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Error("Couldn't accept client connection:", err)
			closeConnection(clientConn)
			continue
		}

		clientSession, err := smux.Server(clientConn, nil)
		if err != nil {
			log.Error("Couldn't create smux session:", err)
			closeConnection(clientConn)
			continue
		}

		go s.handleSession(clientConn, clientSession)
	}
}
