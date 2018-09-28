package chord

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	nlog "github.com/nknorg/nkn/util/log"
)

/*
TCPTransport provides a TCP based Chord transport layer. This allows Chord
to be implemented over a network, instead of only using the LocalTransport. It is
meant to be a simple implementation, optimizing for simplicity instead of performance.
Messages are sent with a header frame, followed by a body frame. All data is encoded
using the GOB format for simplicity.

Internally, there is 1 Goroutine listening for inbound connections, 1 Goroutine PER
inbound connection.
*/
type TCPTransport struct {
	sock           *net.TCPListener
	timeout        time.Duration
	maxIdle        time.Duration
	lock           sync.RWMutex
	local          map[string]*localRPC
	inbound        map[*net.TCPConn]time.Time
	inConnQue      chan struct{}
	pool           map[string]chan *tcpOutConn
	poolLock       sync.RWMutex
	outConnQue     map[string]chan struct{}
	outConnQueLock sync.RWMutex
	shutdown       int32
}

type tcpOutConn struct {
	host   string
	sock   *net.TCPConn
	header tcpHeader
	enc    *gob.Encoder
	dec    *gob.Decoder
	used   time.Time
}

const (
	_ = iota // request type = 0 cannot be decoded correctly so we start from 1
	tcpPing
	tcpListReq
	tcpGetPredReq
	tcpNotifyReq
	tcpFindSucReq
	tcpClearPredReq
	tcpSkipSucReq
)

type tcpHeader struct {
	ReqType int
}

// Potential body types
type tcpBodyError struct {
	Err error
}
type tcpBodyString struct {
	S string
}
type tcpBodyVnode struct {
	Vn *Vnode
}
type tcpBodyTwoVnode struct {
	Target *Vnode
	Vn     *Vnode
}
type tcpBodyFindSuc struct {
	Target *Vnode
	Num    int
	Key    []byte
}
type tcpBodyVnodeError struct {
	Vnode *Vnode
	Err   error
}
type tcpBodyVnodeListError struct {
	Vnodes []*Vnode
	Err    error
}
type tcpBodyBoolError struct {
	B   bool
	Err error
}

const maxOutConnPool = 4
const maxOutConnPerHost = 20
const maxInboundConns = 1000

// Creates a new TCP transport on the given listen address with the
// configured timeout duration.
func InitTCPTransport(listen string, timeout time.Duration) (*TCPTransport, error) {
	// Try to start the listener
	sock, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, err
	}

	// Setup the transport
	tcp := &TCPTransport{
		sock:       sock.(*net.TCPListener),
		timeout:    timeout,
		maxIdle:    time.Duration(60 * time.Second), // Maximum age of a connection
		local:      make(map[string]*localRPC),      // allocate maps
		inbound:    make(map[*net.TCPConn]time.Time),
		inConnQue:  make(chan struct{}, maxInboundConns), // Quota of sock. TODO: get quota from config
		pool:       make(map[string]chan *tcpOutConn),
		outConnQue: make(map[string]chan struct{}),
	}

	// Listen for connections
	go tcp.listen()

	// Reap old connections
	go tcp.reapOld()

	// Done
	return tcp, nil
}

// Checks for a local vnode
func (t *TCPTransport) get(vn *Vnode) (VnodeRPC, bool) {
	if vn == nil {
		return nil, false
	}
	key := vn.String()
	t.lock.RLock()
	defer t.lock.RUnlock()
	w, ok := t.local[key]
	if ok && w != nil && w.obj != nil {
		return w.obj, ok
	} else {
		return nil, false
	}
}

// Gets an outbound connection to a host
func (t *TCPTransport) getConn(host string) (*tcpOutConn, error) {
	if atomic.LoadInt32(&t.shutdown) == 1 {
		return nil, fmt.Errorf("TCP transport is shutdown")
	}

	t.poolLock.RLock()
	outPool, ok := t.pool[host]
	t.poolLock.RUnlock()

	if !ok {
		outPool = make(chan *tcpOutConn, maxOutConnPool)
		t.poolLock.Lock()
		t.pool[host] = outPool
		t.poolLock.Unlock()
	}

	t.outConnQueLock.RLock()
	outChan, ok := t.outConnQue[host]
	t.outConnQueLock.RUnlock()

	if !ok {
		outChan = make(chan struct{}, maxOutConnPerHost)
		t.outConnQueLock.Lock()
		t.outConnQue[host] = outChan
		t.outConnQueLock.Unlock()
	}

	var out *tcpOutConn
	timeoutChan := make(chan struct{}, 1)

	time.AfterFunc(t.timeout, func() {
		timeoutChan <- struct{}{}
	})

	for {
		select {
		case out = <-outPool:
			if out == nil {
				return nil, fmt.Errorf("Out connection is nil")
			}
			_, err := out.sock.Read(nil)
			if err == nil {
				return out, nil
			}
			t.closeOutConn(out)
		default:
		}

		select {
		case outChan <- struct{}{}:
			// Try to establish a connection
			conn, err := net.DialTimeout("tcp", host, t.timeout)
			if err != nil {
				<-outChan
				return nil, err
			}

			// Setup the socket
			sock := conn.(*net.TCPConn)
			t.setupConn(sock)
			enc := gob.NewEncoder(sock)
			dec := gob.NewDecoder(sock)
			now := time.Now()

			// Wrap the sock
			out = &tcpOutConn{host: host, sock: sock, enc: enc, dec: dec, used: now}
			return out, nil
		default:
		}

		select {
		case <-timeoutChan:
			return nil, fmt.Errorf("Get connection timeout to %s", host)
		default:
		}

		<-time.After(100 * time.Millisecond)
	}
}

// Returns an outbound TCP connection to the pool
func (t *TCPTransport) returnConn(o *tcpOutConn, success bool) {
	// Update the last used time
	if success {
		o.used = time.Now()
	}

	// Push back into the pool
	if atomic.LoadInt32(&t.shutdown) == 1 {
		t.closeOutConn(o)
		return
	}

	t.poolLock.RLock()
	outPool := t.pool[o.host]
	t.poolLock.RUnlock()

	select {
	case outPool <- o:
	default:
		nlog.Infof("Connection pool to %s full, close conn.", o.host)
		t.closeOutConn(o)
	}
}

// Setup a connection
func (t *TCPTransport) setupConn(c *net.TCPConn) {
	// c.SetNoDelay(true)
	// c.SetKeepAlive(true)
}

func (t *TCPTransport) closeOutConn(o *tcpOutConn) {
	o.sock.Close()
	t.outConnQueLock.RLock()
	<-t.outConnQue[o.host]
	t.outConnQueLock.RUnlock()
}

// Gets a list of the vnodes on the box
func (t *TCPTransport) ListVnodes(host string) ([]*Vnode, error) {
	// Get a conn
	out, err := t.getConn(host)
	if err != nil {
		return nil, err
	}

	// Response channels
	respChan := make(chan []*Vnode, 1)
	errChan := make(chan error, 1)

	go func() {
		// Send a list command
		out.header.ReqType = tcpListReq
		body := tcpBodyString{S: host}
		if err := out.enc.Encode(&out.header); err != nil {
			errChan <- err
			return
		}
		if err := out.enc.Encode(&body); err != nil {
			errChan <- err
			return
		}

		// Read in the response
		resp := tcpBodyVnodeListError{}
		if err := out.dec.Decode(&resp); err != nil {
			errChan <- err
		}

		if resp.Err == nil {
			respChan <- resp.Vnodes
		} else {
			errChan <- resp.Err
		}
	}()

	select {
	case <-time.After(t.timeout):
		t.returnConn(out, false)
		return nil, fmt.Errorf("ListVnodes timed out while connecting %s!", host)
	case err := <-errChan:
		t.returnConn(out, false)
		return nil, err
	case res := <-respChan:
		t.returnConn(out, true)
		return res, nil
	}
}

// Ping a Vnode, check for liveness
func (t *TCPTransport) Ping(vn *Vnode) (bool, error) {
	// Get a conn
	out, err := t.getConn(vn.Host)
	if err != nil {
		return false, nil
	}

	// Response channels
	respChan := make(chan bool, 1)
	errChan := make(chan error, 1)

	go func() {
		// Send a list command
		out.header.ReqType = tcpPing
		body := tcpBodyVnode{Vn: vn}
		if err := out.enc.Encode(&out.header); err != nil {
			errChan <- err
			return
		}
		if err := out.enc.Encode(&body); err != nil {
			errChan <- err
			return
		}

		// Read in the response
		resp := tcpBodyBoolError{}
		if err := out.dec.Decode(&resp); err != nil {
			errChan <- err
			return
		}

		if resp.Err == nil {
			respChan <- resp.B
		} else {
			errChan <- resp.Err
		}
	}()

	select {
	case <-time.After(t.timeout):
		t.returnConn(out, false)
		return false, fmt.Errorf("Ping timed out while connecting %s!", vn.Host)
	case err := <-errChan:
		t.returnConn(out, false)
		return false, err
	case res := <-respChan:
		t.returnConn(out, true)
		return res, nil
	}
}

// Request a nodes predecessor
func (t *TCPTransport) GetPredecessor(vn *Vnode) (*Vnode, error) {
	// Get a conn
	out, err := t.getConn(vn.Host)
	if err != nil {
		return nil, err
	}

	respChan := make(chan *Vnode, 1)
	errChan := make(chan error, 1)

	go func() {
		// Send a list command
		out.header.ReqType = tcpGetPredReq
		body := tcpBodyVnode{Vn: vn}
		if err := out.enc.Encode(&out.header); err != nil {
			errChan <- err
			return
		}
		if err := out.enc.Encode(&body); err != nil {
			errChan <- err
			return
		}

		// Read in the response
		resp := tcpBodyVnodeError{}
		if err := out.dec.Decode(&resp); err != nil {
			errChan <- err
			return
		}

		if resp.Err == nil {
			respChan <- resp.Vnode
		} else {
			errChan <- resp.Err
		}
	}()

	select {
	case <-time.After(t.timeout):
		t.returnConn(out, false)
		return nil, fmt.Errorf("GetPredecessor timed out while connecting %s!", vn.Host)
	case err := <-errChan:
		t.returnConn(out, false)
		return nil, err
	case res := <-respChan:
		t.returnConn(out, true)
		return res, nil
	}
}

// Notify our successor of ourselves
func (t *TCPTransport) Notify(target, self *Vnode) ([]*Vnode, error) {
	// Get a conn
	out, err := t.getConn(target.Host)
	if err != nil {
		return nil, err
	}

	respChan := make(chan []*Vnode, 1)
	errChan := make(chan error, 1)

	go func() {
		// Send a list command
		out.header.ReqType = tcpNotifyReq
		body := tcpBodyTwoVnode{Target: target, Vn: self}
		if err := out.enc.Encode(&out.header); err != nil {
			errChan <- err
			return
		}
		if err := out.enc.Encode(&body); err != nil {
			errChan <- err
			return
		}

		// Read in the response
		resp := tcpBodyVnodeListError{}
		if err := out.dec.Decode(&resp); err != nil {
			errChan <- err
			return
		}

		if resp.Err == nil {
			respChan <- resp.Vnodes
		} else {
			errChan <- resp.Err
		}
	}()

	select {
	case <-time.After(t.timeout):
		t.returnConn(out, false)
		return nil, fmt.Errorf("Notify timed out while connecting %s!", target.Host)
	case err := <-errChan:
		t.returnConn(out, false)
		return nil, err
	case res := <-respChan:
		t.returnConn(out, true)
		return res, nil
	}
}

// Find a successor
func (t *TCPTransport) FindSuccessors(vn *Vnode, n int, k []byte) ([]*Vnode, error) {
	// Get a conn
	out, err := t.getConn(vn.Host)
	if err != nil {
		return nil, err
	}

	respChan := make(chan []*Vnode, 1)
	errChan := make(chan error, 1)

	go func() {
		// Send a list command
		out.header.ReqType = tcpFindSucReq
		body := tcpBodyFindSuc{Target: vn, Num: n, Key: k}
		if err := out.enc.Encode(&out.header); err != nil {
			errChan <- err
			return
		}
		if err := out.enc.Encode(&body); err != nil {
			errChan <- err
			return
		}

		// Read in the response
		resp := tcpBodyVnodeListError{}
		if err := out.dec.Decode(&resp); err != nil {
			errChan <- err
			return
		}

		if resp.Err == nil {
			respChan <- resp.Vnodes
		} else {
			errChan <- resp.Err
		}
	}()

	select {
	case <-time.After(t.timeout):
		t.returnConn(out, false)
		return nil, fmt.Errorf("FindSuccessors timed out while connecting %s!", vn.Host)
	case err := <-errChan:
		t.returnConn(out, false)
		return nil, err
	case res := <-respChan:
		t.returnConn(out, true)
		return res, nil
	}
}

// Clears a predecessor if it matches a given vnode. Used to leave.
func (t *TCPTransport) ClearPredecessor(target, self *Vnode) error {
	// Get a conn
	out, err := t.getConn(target.Host)
	if err != nil {
		return err
	}

	respChan := make(chan bool, 1)
	errChan := make(chan error, 1)

	go func() {
		// Send a list command
		out.header.ReqType = tcpClearPredReq
		body := tcpBodyTwoVnode{Target: target, Vn: self}
		if err := out.enc.Encode(&out.header); err != nil {
			errChan <- err
			return
		}
		if err := out.enc.Encode(&body); err != nil {
			errChan <- err
			return
		}

		// Read in the response
		resp := tcpBodyError{}
		if err := out.dec.Decode(&resp); err != nil {
			errChan <- err
			return
		}

		if resp.Err == nil {
			respChan <- true
		} else {
			errChan <- resp.Err
		}
	}()

	select {
	case <-time.After(t.timeout):
		t.returnConn(out, false)
		return fmt.Errorf("ClearPredecessor timed out while connecting %s!", target.Host)
	case err := <-errChan:
		t.returnConn(out, false)
		return err
	case <-respChan:
		t.returnConn(out, true)
		return nil
	}
}

// Instructs a node to skip a given successor. Used to leave.
func (t *TCPTransport) SkipSuccessor(target, self *Vnode) error {
	// Get a conn
	out, err := t.getConn(target.Host)
	if err != nil {
		return err
	}

	respChan := make(chan bool, 1)
	errChan := make(chan error, 1)

	go func() {
		// Send a list command
		out.header.ReqType = tcpSkipSucReq
		body := tcpBodyTwoVnode{Target: target, Vn: self}
		if err := out.enc.Encode(&out.header); err != nil {
			errChan <- err
			return
		}
		if err := out.enc.Encode(&body); err != nil {
			errChan <- err
			return
		}

		// Read in the response
		resp := tcpBodyError{}
		if err := out.dec.Decode(&resp); err != nil {
			errChan <- err
			return
		}

		if resp.Err == nil {
			respChan <- true
		} else {
			errChan <- resp.Err
		}
	}()

	select {
	case <-time.After(t.timeout):
		t.returnConn(out, false)
		return fmt.Errorf("SkipSuccessor timed out while connecting %s!", target.Host)
	case err := <-errChan:
		t.returnConn(out, false)
		return err
	case <-respChan:
		t.returnConn(out, true)
		return nil
	}
}

// Register for an RPC callbacks
func (t *TCPTransport) Register(v *Vnode, o VnodeRPC) {
	key := v.String()
	t.lock.Lock()
	t.local[key] = &localRPC{v, o}
	t.lock.Unlock()
}

// Shutdown the TCP transport
func (t *TCPTransport) Shutdown() {
	atomic.StoreInt32(&t.shutdown, 1)
	t.sock.Close()

	// Close all the inbound connections
	t.lock.RLock()
	for conn := range t.inbound {
		conn.Close()
	}
	t.lock.RUnlock()

	// Close all the outbound
	t.poolLock.Lock()
	for _, conns := range t.pool {
		close(conns)
		for out := range conns {
			t.closeOutConn(out)
		}
	}
	t.pool = nil
	t.poolLock.Unlock()
}

// Closes old outbound connections
func (t *TCPTransport) reapOld() {
	for {
		if atomic.LoadInt32(&t.shutdown) == 1 {
			return
		}
		time.Sleep(10 * time.Second)
		t.reapOnce()
	}
}

func (t *TCPTransport) reapOnce() {
	t.poolLock.Lock()
	for host, conns := range t.pool {
		if len(conns) == 0 {
			continue
		}

		// close non neighbors
		if ring != nil && !ring.shouldConnectToHost(host) {
			log.Printf("[INFO] Disconnect with %d chord nodes at %s.", len(conns), host)
			close(conns)
			for conn := range conns {
				t.closeOutConn(conn)
			}
			t.pool[host] = make(chan *tcpOutConn, maxOutConnPool)
			continue
		}

		// check timeout
		select {
		case o := <-conns:
			if time.Since(o.used) > t.maxIdle {
				log.Printf("[INFO] Disconnect with idle chord node at %s.", host)
				t.closeOutConn(o)
			} else {
				select {
				case conns <- o:
				default:
					t.closeOutConn(o)
				}
			}
		default:
		}
	}
	t.poolLock.Unlock()

	t.lock.Lock()
	for conn, lastUsed := range t.inbound {
		if time.Since(lastUsed) > t.maxIdle {
			log.Printf("[INFO] Close timeout inbound connection with %s.", conn.RemoteAddr().String())
			delete(t.inbound, conn)
			conn.Close()
		}
	}
	t.lock.Unlock()
}

// Listens for inbound connections
func (t *TCPTransport) listen() {
	for {
		conn, err := t.sock.AcceptTCP()
		if err != nil {
			if atomic.LoadInt32(&t.shutdown) == 0 {
				log.Printf("[ERR] Error accepting TCP connection! %s\n", err)
				time.Sleep(time.Millisecond * 300) // Add delay before try again sock Accept
				continue
			} else {
				return
			}
		}

		// Setup the conn
		t.setupConn(conn)

		// Register the inbound conn
		t.lock.Lock()
		t.inbound[conn] = time.Now()
		t.lock.Unlock()

		if len(t.inConnQue) >= maxInboundConns {
			log.Printf("[WARN] The quota reach the limitation %d\n", len(t.inConnQue))
		}

		t.inConnQue <- struct{}{} // Push
		go t.handleConn(conn)     // Start handler
	}
}

// Handles inbound TCP connections
func (t *TCPTransport) handleConn(conn *net.TCPConn) {
	// Defer the cleanup
	defer func() {
		t.lock.Lock()
		delete(t.inbound, conn)
		t.lock.Unlock()
		conn.Close()
		<-t.inConnQue // Pop
	}()

	dec := gob.NewDecoder(conn)
	enc := gob.NewEncoder(conn)
	header := tcpHeader{}
	var sendResp interface{}
	for {
		// Get the header
		if err := dec.Decode(&header); err != nil {
			if atomic.LoadInt32(&t.shutdown) == 0 && err.Error() != "EOF" {
				log.Printf("[ERR] Failed to decode TCP header! Got %s", err)
			}
			return
		}

		// Read in the body and process request
		switch header.ReqType {
		case tcpPing:
			body := tcpBodyVnode{}
			if err := dec.Decode(&body); err != nil {
				log.Printf("[ERR] Failed to decode TCP body! Got %s", err)
				return
			}

			// Generate a response
			_, ok := t.get(body.Vn)
			if ok {
				sendResp = tcpBodyBoolError{B: ok, Err: nil}
			} else {
				sendResp = tcpBodyBoolError{B: ok, Err: fmt.Errorf("Target VN not found! Target %s:%s",
					body.Vn.Host, body.Vn.String())}
				log.Printf("[ERR] Target VN not found! Target %s:%s",
					body.Vn.Host, body.Vn.String())
			}

		case tcpListReq:
			body := tcpBodyString{}
			if err := dec.Decode(&body); err != nil {
				log.Printf("[ERR] Failed to decode TCP body! Got %s", err)
				return
			}

			// Generate all the local clients
			res := make([]*Vnode, 0, len(t.local))

			// Build list
			t.lock.RLock()
			for _, v := range t.local {
				res = append(res, v.vnode)
			}
			t.lock.RUnlock()

			// Make response
			sendResp = tcpBodyVnodeListError{Vnodes: trimSlice(res)}

		case tcpGetPredReq:
			body := tcpBodyVnode{}
			if err := dec.Decode(&body); err != nil {
				log.Printf("[ERR] Failed to decode TCP body! Got %s", err)
				return
			}

			// Generate a response
			obj, ok := t.get(body.Vn)
			resp := tcpBodyVnodeError{}
			sendResp = &resp
			if ok {
				node, err := obj.GetPredecessor()
				resp.Vnode = node
				resp.Err = err
			} else {
				resp.Err = fmt.Errorf("Target VN not found! Target %s:%s",
					body.Vn.Host, body.Vn.String())
			}
			if resp.Err != nil {
				log.Println("[ERR]", resp.Err)
			}

		case tcpNotifyReq:
			body := tcpBodyTwoVnode{}
			if err := dec.Decode(&body); err != nil {
				log.Printf("[ERR] Failed to decode TCP body! Got %s", err)
				return
			}
			if body.Target == nil {
				return
			}

			// Generate a response
			obj, ok := t.get(body.Target)
			resp := tcpBodyVnodeListError{}
			sendResp = &resp
			if ok {
				nodes, err := obj.Notify(body.Vn)
				resp.Vnodes = trimSlice(nodes)
				resp.Err = err
			} else {
				resp.Err = fmt.Errorf("Target VN not found! Target %s:%s",
					body.Target.Host, body.Target.String())
			}
			if resp.Err != nil {
				log.Println("[ERR]", resp.Err)
			}

		case tcpFindSucReq:
			body := tcpBodyFindSuc{}
			if err := dec.Decode(&body); err != nil {
				log.Printf("[ERR] Failed to decode TCP body! Got %s", err)
				return
			}

			// Generate a response
			obj, ok := t.get(body.Target)
			resp := tcpBodyVnodeListError{}
			sendResp = &resp
			if ok && obj != nil {
				nodes, err := obj.FindSuccessors(body.Num, body.Key)
				resp.Vnodes = trimSlice(nodes)
				resp.Err = err
			} else {
				resp.Err = fmt.Errorf("Target VN not found! Target %s:%s",
					body.Target.Host, body.Target.String())
			}
			if resp.Err != nil {
				log.Println("[ERR]", resp.Err)
			}

		case tcpClearPredReq:
			body := tcpBodyTwoVnode{}
			if err := dec.Decode(&body); err != nil {
				log.Printf("[ERR] Failed to decode TCP body! Got %s", err)
				return
			}

			// Generate a response
			obj, ok := t.get(body.Target)
			resp := tcpBodyError{}
			sendResp = &resp
			if ok {
				resp.Err = obj.ClearPredecessor(body.Vn)
			} else {
				resp.Err = fmt.Errorf("Target VN not found! Target %s:%s",
					body.Target.Host, body.Target.String())
			}
			if resp.Err != nil {
				log.Println("[ERR]", resp.Err)
			}

		case tcpSkipSucReq:
			body := tcpBodyTwoVnode{}
			if err := dec.Decode(&body); err != nil {
				log.Printf("[ERR] Failed to decode TCP body! Got %s", err)
				return
			}

			// Generate a response
			obj, ok := t.get(body.Target)
			resp := tcpBodyError{}
			sendResp = &resp
			if ok {
				resp.Err = obj.SkipSuccessor(body.Vn)
			} else {
				resp.Err = fmt.Errorf("Target VN not found! Target %s:%s",
					body.Target.Host, body.Target.String())
			}
			if resp.Err != nil {
				log.Println("[ERR]", resp.Err)
			}

		default:
			log.Printf("[ERR] Unknown request type! Got %d", header.ReqType)
			return
		}

		// Send the response
		if err := enc.Encode(sendResp); err != nil {
			log.Printf("[ERR] Failed to send TCP body! Got %s", err.Error())
			return
		}

		t.lock.Lock()
		t.inbound[conn] = time.Now()
		t.lock.Unlock()
	}
}

// Trims the slice to remove nil elements
func trimSlice(vn []*Vnode) []*Vnode {
	if vn == nil {
		return vn
	}

	// Find a non-nil index
	idx := len(vn) - 1
	for idx >= 0 && vn[idx] == nil {
		idx--
	}
	return vn[:idx+1]
}
