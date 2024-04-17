package server

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	api "github.com/nknorg/nkn/v2/api/common"
	"github.com/nknorg/nkn/v2/api/ratelimiter"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/util/log"
)

// type conn interface {
// 	Start(s *MsgServer, wssCertReady chan struct{}) error
// }

type wsServer struct {
	s           *MsgServer
	Upgrader    websocket.Upgrader
	listener    net.Listener
	tlsListener net.Listener
	server      *http.Server
	tlsServer   *http.Server
}

func (ws *wsServer) Start(s *MsgServer, wssCertReady chan struct{}) error {
	ws.s = s
	if config.Parameters.HttpWsPort == 0 {
		log.Error("Not configure HttpWsPort port ")
		return nil
	}
	ws.Upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	var err error

	ws.listener, err = net.Listen("tcp", ":"+strconv.Itoa(int(config.Parameters.HttpWsPort)))
	if err != nil {
		log.Error("net.Listen: ", err.Error())
		return err
	}

	ws.server = &http.Server{Handler: http.HandlerFunc(ws.websocketHandler)}
	go ws.server.Serve(ws.listener)

	go func(wssCertReady chan struct{}) {
		if wssCertReady == nil {
			return
		}
		for {
			select {
			case <-wssCertReady:
				log.Info("wss cert received")
				ws.tlsListener, err = ws.initTlsListen()
				if err != nil {
					log.Error("Https Cert: ", err.Error())
				}
				err = ws.server.Serve(ws.tlsListener)
				if err != nil {
					log.Error(err)
				}
				return
			case <-time.After(300 * time.Second):
				log.Info("wss server is unavailable yet")
			}
		}
	}(wssCertReady)

	return nil
}

func (ws *wsServer) websocketHandler(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		limiter := ratelimiter.GetLimiter("ws:"+host, config.Parameters.WsIPRateLimit, int(config.Parameters.WsIPRateBurst))
		if !limiter.Allow() {
			log.Infof("Ws connection limit of %s reached", host)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
	}

	wsServer, err := ws.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("websocket Upgrader: ", err)
		return
	}
	defer wsServer.Close()

	ws.s.newConnection(wsServer, r)
}

func (ws *wsServer) initTlsListen() (net.Listener, error) {
	tlsConfig := &tls.Config{
		GetCertificate: api.GetWssCertificate,
	}

	listener, err := tls.Listen("tcp", ":"+strconv.Itoa(int(config.Parameters.HttpWssPort)), tlsConfig)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return listener, nil
}

func (ws *wsServer) Stop() {
	if ws.server != nil {
		ws.server.Shutdown(context.Background())
		log.Error("Close websocket ")
	}
}
