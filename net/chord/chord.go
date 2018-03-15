/* Package chord

This package is a collection of structures and functions associated
with the Chord distributed lookup protocol.
*/
package chord

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"os"
	"time"
)

//Finger type denoting identifying information about a ChordNode
type Finger struct {
	id     [sha256.Size]byte
	ipaddr string
}

type request struct {
	write bool
	succ  bool
	index int
}

//ChordNode type denoting a Chord server.
type ChordNode struct {
	predecessor   *Finger
	successor     *Finger
	successorList [sha256.Size * 8]Finger
	fingerTable   [sha256.Size*8 + 1]Finger

	finger  chan Finger
	request chan request

	id     [sha256.Size]byte
	ipaddr string

	connections  map[string]net.TCPConn
	applications map[byte]ChordApp

	//testing purposes only
	malicious byte
}

type PeerError struct {
	Address string
	Err     error
}

func (e *PeerError) Error() string {
	return fmt.Sprintf("Failed to connect to peer: %s. Cause of failure: %s.", e.Address, e.Err)
}

//error checking function
func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
	}
}

//Lookup returns the address of the successor of key in the Chord DHT.
//The lookup process is iterative. Beginning with the address of a
//Chord node, start, this function will request the finger tables of
//the closest preceeding Chord node to key until the successor is found.
//
//If the start address is unreachable, the error is of type PeerError.
func Lookup(key [sha256.Size]byte, start string) (addr string, err error) {

	addr = start

	msg := getfingersMsg()
	reply, err := Send(msg, start)
	if err != nil { //node failed.
		err = &PeerError{start, err}
		return
	}

	ft, err := parseFingers(reply)
	if err != nil {
		err = &PeerError{start, err}
		return
	}
	if len(ft) < 2 {
		return
	}

	current := ft[0]

	if key == current.id {
		addr = current.ipaddr
		return
	}

	//loop through finger table and see what the closest finger is
	for i := len(ft) - 1; i > 0; i-- {
		f := ft[i]
		if i == 0 {
			break
		}
		if InRange(f.id, current.id, key) { //see if f.id is closer than I am.
			addr, err = Lookup(key, f.ipaddr)
			if err != nil { //node failed
				continue
			}
			return
		}
	}
	addr = ft[1].ipaddr
	msg = pingMsg()
	reply, err = Send(msg, addr)

	//this code is executed if the current node's successor has gone missing
	if err != nil {
		//ask node for its successor list
		msg = getsuccessorsMsg()
		reply, err = Send(msg, current.ipaddr)
		if err != nil {
			addr = current.ipaddr
			return
		}

		ft, err = parseFingers(reply)
		if err != nil {
			addr = current.ipaddr
			return
		}

		for i := 0; i < len(ft); i++ {
			f := ft[i]
			if i == 0 {
				break
			}
			msg = pingMsg()
			reply, err = Send(msg, f.ipaddr)
			if err != nil { //closest next successor that responds
				addr = f.ipaddr
				return
			}
		}

		addr = current.ipaddr
		return
	}

	return
}

//Lookup returns the address of the ChordNode that is responsible
//for the key. The procedure begins at the address denoted by start.
func (node *ChordNode) lookup(key [sha256.Size]byte, start string) (addr string, err error) {

	addr = start

	msg := getfingersMsg()
	reply, err := node.send(msg, start)
	if err != nil { //node failed
		err = &PeerError{start, err}
		return
	}

	ft, err := parseFingers(reply)
	if err != nil {
		err = &PeerError{start, err}
		return
	}
	if len(ft) < 2 {
		return
	}

	current := ft[0]

	if key == current.id {
		addr = current.ipaddr
		return
	}

	//loop through finger table and see what the closest finger is
	for i := len(ft) - 1; i > 0; i-- {
		f := ft[i]
		if i == 0 {
			break
		}
		if InRange(f.id, current.id, key) { //see if f.id is closer than I am.
			addr, err = node.lookup(key, f.ipaddr)
			if err != nil { //node failed
				continue
			}
			return
		}
	}
	addr = ft[1].ipaddr
	msg = pingMsg()
	reply, err = node.send(msg, addr)

	//this code is executed if the id's successor has gone missing
	if err != nil {
		//ask node for its successor list
		msg = getsuccessorsMsg()
		reply, err = node.send(msg, current.ipaddr)
		if err != nil {
			addr = current.ipaddr
			return
		}
		ft, err = parseFingers(reply)
		if err != nil {
			addr = current.ipaddr
			return
		}

		for i := 0; i < len(ft); i++ {
			f := ft[i]
			if i == 0 {
				break
			}
			msg = pingMsg()
			reply, err = node.send(msg, f.ipaddr)
			if err != nil { //closest next successor that responds
				addr = f.ipaddr
				return
			}
		}

		addr = current.ipaddr
		return
	}

	return
}

//Create will start a new Chord DHT and return the original ChordNode
func Create(myaddr string) *ChordNode {
	node := new(ChordNode)
	//initialize node information
	node.id = sha256.Sum256([]byte(myaddr))
	node.ipaddr = myaddr
	me := new(Finger)
	me.id = node.id
	me.ipaddr = node.ipaddr
	node.fingerTable[0] = *me
	succ := new(Finger)
	node.successor = succ
	pred := new(Finger)
	node.predecessor = pred

	//set up channels for finger manager
	c := make(chan Finger)
	c2 := make(chan request)
	node.finger = c
	node.request = c2

	//initialize listener and network manager threads
	node.listen(myaddr)
	node.connections = make(map[string]net.TCPConn)
	node.applications = make(map[byte]ChordApp)

	//initialize maintenance and finger manager threads
	go node.data()
	go node.maintain()
	return node
}

//Join will add a new ChordNode to an existing DHT. It looks up the successor
//of the new node starting at an existing Chord node specified by addr. Join
//returns the new ChordNode when completed.
//
//If the start address is unreachable, the error is of type PeerError.
func Join(myaddr string, addr string) (*ChordNode, error) {
	node := Create(myaddr)
	successor, err := Lookup(node.id, addr)
	if err != nil || successor == "" {
		return nil, &PeerError{addr, err}
	}

	//find id of node
	msg := getidMsg()
	reply, err := Send(msg, successor)
	if err != nil {
		return nil, &PeerError{addr, err}
	}

	//update node info to include successor
	succ := new(Finger)
	succ.id, err = parseId(reply)
	if err != nil {
		return nil, &PeerError{addr, err}
	}
	succ.ipaddr = successor
	node.query(true, false, 1, succ)

	return node, nil
}

//data manages reads and writes to the node data structure
func (node *ChordNode) data() {
	for {
		req := <-node.request
		if req.write {
			if req.succ {
				node.successorList[req.index] = <-node.finger
			} else {
				if req.index < 0 {
					*node.predecessor = <-node.finger
				} else if req.index == 1 {
					*node.successor = <-node.finger
					node.fingerTable[1] = *node.successor
					node.successorList[0] = *node.successor
				} else {
					node.fingerTable[req.index] = <-node.finger
				}
			}
		} else { //req.read
			if req.succ {
				node.finger <- node.successorList[req.index]
			} else {
				if req.index < 0 {
					node.finger <- *node.predecessor
				} else {
					node.finger <- node.fingerTable[req.index]
				}
			}
		}
	}
}

//query allows functions to read from or write to the node object
func (node *ChordNode) query(write bool, succ bool, index int, newf *Finger) Finger {
	f := new(Finger)
	req := request{write, succ, index}
	node.request <- req
	if write {
		node.finger <- *newf
	} else {
		*f = <-node.finger
	}

	return *f
}

//maintain will periodically perform maintenance operations
func (node *ChordNode) maintain() {
	ctr := 0
	for {
		time.Sleep(time.Duration(rand.Uint32()%3)*time.Minute + time.Duration(rand.Uint32()%60)*time.Second + time.Duration(rand.Uint32()%60)*time.Millisecond)
		//stabilize
		node.stabilize()
		//check predecessor
		node.checkPred()
		//update fingers
		node.fix(ctr)
		ctr = ctr % 256
		ctr += 1
	}
}

//stablize ensures that the node's successor's predecessor is itself
//If not, it updates its successor's predecessor.
func (node *ChordNode) stabilize() {
	successor := node.query(false, false, 1, nil)

	if successor.zero() {
		return
	}

	//check to see if successor is still around
	msg := pingMsg()
	reply, err := node.send(msg, successor.ipaddr)
	if err != nil {
		//successor failed to respond
		//check in successor list for next available successor.
		for i := 1; i < sha256.Size*8; i++ {
			successor = node.query(false, true, i, nil)
			if successor.ipaddr == node.ipaddr {
				continue
			}
			msg := pingMsg()
			reply, err = node.send(msg, successor.ipaddr)
			if err == nil {
				break
			} else {
				successor.ipaddr = ""
			}
		}
		node.query(true, false, 1, &successor)
		if successor.ipaddr == "" {
			return
		}
	}

	//everything is OK, update successor list
	msg = getsuccessorsMsg()
	reply, err = node.send(msg, successor.ipaddr)
	if err != nil {
		return
	}
	ft, err := parseFingers(reply)
	if err != nil {
		return
	}
	for i := range ft {
		if i < sha256.Size*8-1 {
			node.query(true, true, i+1, &ft[i])
		}
	}

	//ask sucessor for predecessor
	msg = getpredMsg()
	reply, err = node.send(msg, successor.ipaddr)
	if err != nil {
		return
	}

	predOfSucc, err := parseFinger(reply)
	if err != nil { //node failed
		return
	}
	if predOfSucc.ipaddr != "" {
		if predOfSucc.id != node.id {
			if InRange(predOfSucc.id, node.id, successor.id) {
				node.query(true, false, 1, &predOfSucc)
			}
		} else { //everything is fine
			return
		}
	}

	//claim to be predecessor of succ
	me := new(Finger)
	me.id = node.id
	me.ipaddr = node.ipaddr
	msg = claimpredMsg(*me)
	node.send(msg, successor.ipaddr)

}

//Register allows chord applications to register themselves and receive notifications
//and messages through the Chord DHT.
//
//A Chord node registers an application app and forwards all messages with the
//identifier id by calling the interface method Message. Applications will also
//be notified of any changes in the underlying node's predecessor.
func (node *ChordNode) Register(id byte, app ChordApp) bool {
	if _, ok := node.applications[id]; ok {
		return false
	}
	node.applications[id] = app
	return true

}

func (node *ChordNode) notify(newPred Finger) {
	node.query(true, false, -1, &newPred)
	//update predecessor
	successor := node.query(false, false, 1, nil)
	if successor.zero() { //TODO: so if you get here, you were probably the first node.
		node.query(true, false, 1, &newPred)
	}
	//notify applications
	for _, app := range node.applications {
		app.Notify(newPred.id, node.id, newPred.ipaddr)
	}
}

func (node *ChordNode) checkPred() {
	predecessor := node.query(false, false, -1, nil)
	if predecessor.zero() {
		return
	}

	msg := pingMsg()
	reply, err := node.send(msg, predecessor.ipaddr)
	if err != nil {
		predecessor.ipaddr = ""
		node.query(true, false, -1, &predecessor)
	}

	if success, err := parsePong(reply); !success || err != nil {
		predecessor.ipaddr = ""
		node.query(true, false, -1, &predecessor)
	}

	return

}

func (node *ChordNode) fix(which int) {
	successor := node.query(false, false, 1, nil)
	if which == 0 || which == 1 || successor.zero() {
		return
	}
	var targetId [sha256.Size]byte
	copy(targetId[:sha256.Size], target(node.id, which)[:sha256.Size])
	newip, err := node.lookup(targetId, successor.ipaddr)
	if err != nil { //node failed: TODO make more robust
		checkError(err)
		return
	}
	if newip == node.ipaddr {
		checkError(err)
		return
	}

	//find id of node
	msg := getidMsg()
	reply, err := node.send(msg, newip)
	if err != nil {
		checkError(err)
		return
	}

	newfinger := new(Finger)
	newfinger.ipaddr = newip
	newfinger.id, _ = parseId(reply)
	node.query(true, false, which, newfinger)

}

//Finalize stops all communication and removes the ChordNode from the DHT.
func (node *ChordNode) Finalize() {
	//send message to all children to terminate

	fmt.Printf("Exiting...\n")
}

//InRange is a helper function that returns true if the value x is between the values (min, max)
func InRange(x [sha256.Size]byte, min [sha256.Size]byte, max [sha256.Size]byte) bool {
	//There are 3 cases: min < x and x < max,
	//x < max and max < min, max < min and min < x
	xint := new(big.Int)
	maxint := new(big.Int)
	minint := new(big.Int)
	xint.SetBytes(x[:sha256.Size])
	minint.SetBytes(min[:sha256.Size])
	maxint.SetBytes(max[:sha256.Size])

	if xint.Cmp(minint) == 1 && maxint.Cmp(xint) == 1 {
		return true
	}

	if maxint.Cmp(xint) == 1 && minint.Cmp(maxint) == 1 {
		return true
	}

	if minint.Cmp(maxint) == 1 && xint.Cmp(minint) == 1 {
		return true
	}

	return false
}

//target returns the target id used by the fix function
func target(me [sha256.Size]byte, which int) []byte {
	meint := new(big.Int)
	meint.SetBytes(me[:sha256.Size])

	baseint := new(big.Int)
	baseint.SetUint64(2)

	powint := new(big.Int)
	powint.SetInt64(int64(which - 1))

	var biggest [sha256.Size + 1]byte
	for i := range biggest {
		biggest[i] = 255
	}

	tmp := new(big.Int)
	tmp.SetInt64(1)

	modint := new(big.Int)
	modint.SetBytes(biggest[:sha256.Size])
	modint.Add(modint, tmp)

	target := new(big.Int)
	target.Exp(baseint, powint, modint)
	target.Add(meint, target)
	target.Mod(target, modint)

	bytes := target.Bytes()
	diff := sha256.Size - len(bytes)
	if diff > 0 {
		tmp := make([]byte, sha256.Size)
		//pad with zeros
		for i := 0; i < diff; i++ {
			tmp[i] = 0
		}
		for i := diff; i < sha256.Size; i++ {
			tmp[i] = bytes[i-diff]
		}
		bytes = tmp
	}
	return bytes[:sha256.Size]
}

func (f Finger) String() string {
	return fmt.Sprintf("%s", f.ipaddr)
}

func (f Finger) zero() bool {
	if f.ipaddr == "" {
		return true
	} else {
		return false
	}
}

/** Printouts of information **/

//String returns a string containing the node's ip address, sucessor, and predecessor.
func (node *ChordNode) String() string {
	var succ, pred string
	successor := node.query(false, false, 1, nil)
	predecessor := node.query(false, false, -1, nil)
	if !successor.zero() {
		succ = successor.String()
	} else {
		succ = "Unknown"
	}
	if !predecessor.zero() {
		pred = predecessor.String()
	} else {
		pred = "Unknown"
	}
	return fmt.Sprintf("%s\t%s\t%s\n", node.ipaddr, succ, pred)
}

//ShowFingers returns a string representation of the ChordNode's finger table.
func (node *ChordNode) ShowFingers() string {
	retval := ""
	finger := new(Finger)
	prevfinger := new(Finger)
	ctr := 0
	for i := 0; i < sha256.Size*8+1; i++ {
		*finger = node.query(false, false, i, nil)
		if !finger.zero() {
			ctr += 1
			if i == 0 || finger.ipaddr != prevfinger.ipaddr {
				retval += fmt.Sprintf("%d %s\n", i, finger.String())
			}
		}
		*prevfinger = *finger
	}
	return retval + fmt.Sprintf("Total fingers: %d.\n", ctr)
}

//ShowSucc returns a string representation of the ChordNode's successor list.
func (node *ChordNode) ShowSucc() string {
	table := ""
	finger := new(Finger)
	prevfinger := new(Finger)
	for i := 0; i < sha256.Size*8; i++ {
		*finger = node.query(false, true, i, nil)
		if finger.ipaddr != "" {
			if i == 0 || finger.ipaddr != prevfinger.ipaddr {
				table += fmt.Sprintf("%s\n", finger.String())
			}
		}
		*prevfinger = *finger
	}
	return table
}

/** Chord application interface and methods **/

//ChordApp is an interface for applications to run on top of a Chord DHT.
type ChordApp interface {

	//Notify will alert the application of changes in the ChordNode's predecessor
	Notify(id [sha256.Size]byte, me [sha256.Size]byte, addr string)

	//Message will forward a message that was received through the DHT to the application
	Message(data []byte) []byte
}
