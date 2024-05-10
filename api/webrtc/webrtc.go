package webrtc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/nknorg/nkn/v2/api/ratelimiter"
	"github.com/nknorg/nkn/v2/api/websocket/session"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/util/log"
	"github.com/pion/webrtc/v4"
)

// compitable to websocket
const (
	UnknownMessage = 0
	TextMessage    = 1
	BinaryMessage  = 2
	CloseMessage   = 8
	PingMessage    = 9
	PongMessage    = 10

	PingData = "ping"
	PongData = "pong"
)

var NewConnection func(conn session.Conn, r *http.Request)

type DataChannelMessage struct {
	messageType int
	data        []byte
}

type Peer struct {
	pc               *webrtc.PeerConnection
	dc               *webrtc.DataChannel
	offer            string
	answer           string
	OnSdp            chan string
	OnMessage        chan DataChannelMessage
	OnOfferConnected chan struct{}

	mutex         sync.RWMutex
	isConnected   bool
	readDeadline  time.Time
	writeDeadline time.Time
	readLimit     int64
	pongHandler   func(string) error
}

func NewPeer(urls []string) *Peer {
	p := &Peer{
		OnSdp:            make(chan string, 1),
		isConnected:      false,
		OnMessage:        make(chan DataChannelMessage, 128),
		OnOfferConnected: make(chan struct{}, 1),
	}

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: urls,
			},
		},
	}
	var err error
	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Error("NewPeerConnection error: ", err)
		return nil
	}

	p.pc = pc
	return p
}

func (c *Peer) Offer(label string) error {
	if c.pc == nil {
		return fmt.Errorf("PeerConnection not available")
	}

	c.pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			localDesc := c.pc.LocalDescription()
			encodedDescr, err := Encode(localDesc)
			if err != nil {
				log.Error("WebRTC OnICECandidate error: ", err)
				return
			}
			c.offer = encodedDescr
			select {
			case c.OnSdp <- encodedDescr:
			default:
			}
		}
	})

	dc, err := c.pc.CreateDataChannel(label, nil)
	if err != nil {
		return err
	}

	dc.OnOpen(func() {
		log.Debugf("Data channel %v has been opened\n", dc.Label())
		select {
		case c.OnOfferConnected <- struct{}{}:
		default:
		}
		c.mutex.Lock()
		defer c.mutex.Unlock()
		c.isConnected = true
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		var dcmsg DataChannelMessage
		if msg.IsString {
			dcmsg.messageType = TextMessage
			if string(msg.Data) == PongData {
				if c.pongHandler != nil {
					c.pongHandler(PongData)
				} else {
					log.Info("Pong handler not set")
				}
				return
			} else if string(msg.Data) == PingData {
				dc.SendText(PongData)
				return
			}
		} else {
			dcmsg.messageType = BinaryMessage
		}
		dcmsg.data = msg.Data
		c.OnMessage <- dcmsg
	})
	dc.OnClose(func() {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		c.isConnected = false
	})

	dc.OnError(func(err error) {
		log.Debugf("Data channel %s error: %s\n", dc.Label(), err.Error())
	})

	offer, err := c.pc.CreateOffer(&webrtc.OfferOptions{ICERestart: false})
	if err != nil {
		return nil
	}
	if err = c.pc.SetLocalDescription(offer); err != nil {
		return nil
	}

	c.dc = dc
	return nil
}

func (c *Peer) Answer(offerSdp string) error {
	offer := webrtc.SessionDescription{}
	err := Decode(offerSdp, &offer)
	if err != nil {
		return err
	}

	sdp, err := offer.Unmarshal()
	if err != nil {
		return err
	}
	limiter := ratelimiter.GetLimiter("webrtc:"+sdp.Origin.UnicastAddress, config.Parameters.WsIPRateLimit, int(config.Parameters.WsIPRateBurst))
	if !limiter.Allow() {
		return fmt.Errorf("webrtc connection limit of %s reached", sdp.Origin.UnicastAddress)
	}

	if err := c.pc.SetRemoteDescription(offer); err != nil {
		return err
	}

	answer, err := c.pc.CreateAnswer(nil)
	if err != nil {
		return err
	}

	if err := c.pc.SetLocalDescription(answer); err != nil {
		return err
	}

	c.pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			localDesc := c.pc.LocalDescription()
			encodedDescr, err := Encode(localDesc)
			if err != nil {
				log.Error("WebRTC OnICECandidate error: ", err)
				return
			}
			c.answer = encodedDescr
			select {
			case c.OnSdp <- encodedDescr:
			default:
			}
		}
	})

	c.pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		dc.OnOpen(func() {
			c.dc = dc

			c.mutex.Lock()
			c.isConnected = true
			c.mutex.Unlock()

			if NewConnection != nil {
				go NewConnection(c, nil)
			}
		})

		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			var dcmsg DataChannelMessage
			if msg.IsString {
				dcmsg.messageType = TextMessage
				if string(msg.Data) == PingData {
					dc.SendText(PongData)
					return
				} else if string(msg.Data) == PongData {
					if c.pongHandler != nil {
						c.pongHandler(PongData)
					} else {
						log.Info("Pong handler not set")
					}
					return
				}
			} else {
				dcmsg.messageType = BinaryMessage
			}
			dcmsg.data = msg.Data
			c.OnMessage <- dcmsg
		})

		dc.OnClose(func() {
			c.mutex.Lock()
			c.isConnected = false
			c.mutex.Unlock()
		})

		dc.OnError(func(err error) {
			log.Debugf("Data channel %v error: %s\n", dc.Label(), err.Error())
		})
	})

	return nil
}

func (c *Peer) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.isConnected
}

func (c *Peer) SetRemoteSdp(sdp string) error {
	answer := webrtc.SessionDescription{}
	err := Decode(sdp, &answer)
	if err != nil {
		return err
	}
	return c.pc.SetRemoteDescription(answer)
}

func (c *Peer) WriteMessage(messageType int, data []byte) (err error) {
	if c.dc == nil {
		return fmt.Errorf("DataChannel not available")
	}

	if messageType == PingMessage {
		return c.dc.SendText(PingData)
	}
	if messageType == PongMessage {
		return c.dc.SendText(PongData)
	}
	if data == nil {
		return fmt.Errorf("data to send is nil")
	}

	if c.writeDeadline.IsZero() {
		if messageType == TextMessage {
			err = c.dc.SendText(string(data))
		} else {
			err = c.dc.Send(data)
		}
		return err
	}

	var dur time.Duration
	now := time.Now()
	switch {
	case c.writeDeadline.Before(now), c.writeDeadline == now:
		return os.ErrDeadlineExceeded

	case c.writeDeadline.After(now):
		writeResult := make(chan error)
		go func() {
			if messageType == TextMessage {
				err = c.dc.SendText(string(data))
			} else {
				err = c.dc.Send(data)
			}

			writeResult <- err
		}()

		dur = time.Until(c.writeDeadline)
		select {
		case err = <-writeResult:
			return err
		case <-time.After(dur):
			return os.ErrDeadlineExceeded
		}
	}

	return fmt.Errorf("unknown error")
}

// WriteJSON writes the JSON encoding of v as a message.
func (c *Peer) WriteJSON(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.WriteMessage(TextMessage, b)
}

func (c *Peer) ReadMessage() (messageType int, data []byte, err error) {
	if c.readDeadline.IsZero() {
		msg := <-c.OnMessage
		return msg.messageType, msg.data, nil
	}

	now := time.Now()
	switch {
	case c.readDeadline.After(now):
		for {
			oldReadDeadline := c.readDeadline
			dur := time.Until(c.readDeadline)
			select {
			case msg := <-c.OnMessage:
				return msg.messageType, msg.data, nil
			case <-time.After(dur):
				if c.readDeadline.After(oldReadDeadline) {
					continue
				}
				return UnknownMessage, nil, os.ErrDeadlineExceeded
			}
		}

	case c.readDeadline.Before(now), c.readDeadline == now:
		return UnknownMessage, nil, os.ErrDeadlineExceeded
	}

	return UnknownMessage, nil, fmt.Errorf("unknown error")
}

func (c *Peer) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return nil
}

func (c *Peer) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

func (c *Peer) SetReadLimit(l int64) {
	c.readLimit = l
}

func (c *Peer) Close() error {
	if c.OnSdp != nil {
		close(c.OnSdp)
	}
	if c.OnMessage != nil {
		close(c.OnMessage)
	}
	if c.OnOfferConnected != nil {
		close(c.OnOfferConnected)
	}
	if c.dc != nil {
		if err := c.dc.Close(); err != nil {
			return err
		}
	}
	return c.pc.Close()
}

func (c *Peer) SetPongHandler(f func(string) error) {
	c.pongHandler = f
}

// Encode the input in base64
func Encode(obj interface{}) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}

// Decode the input from base64
func Decode(in string, obj interface{}) error {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, obj)
}
