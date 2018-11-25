package httpproxy

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/nknorg/nkn/util/config"
)

type HTTPProxy struct {
	// defines a listeners for http proxy, such as "127.0.0.1:30004"
	listener string
}

func NewServer() *HTTPProxy {
	return &HTTPProxy{
		listener: ":" + strconv.Itoa(int(config.Parameters.HttpProxyPort)),
	}
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

func (s *HTTPProxy) HandleConnect(w http.ResponseWriter, r *http.Request) {
	dest, err := net.Dial("tcp", r.Host) //TODO: add timeout
	log.Println(r.Method, r.RequestURI, r.Proto)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	src, _, err := hijacker.Hijack()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	go transfer(dest, src)
	go transfer(src, dest)
}

func (s *HTTPProxy) Start() {
	server := &http.Server{
		Addr: s.listener,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodConnect {
				// won't proxy HTTP due to security reasons
				w.WriteHeader(http.StatusForbidden)
				return
			}

			s.HandleConnect(w, r)
		}),

		// HTTP/2 is disabled for now, need to figure out how to proxy correctly and securely
		// connection hijacking isn't possible with http2
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	server.ListenAndServe()
}
