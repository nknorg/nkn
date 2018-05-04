package chord

import (
	"flag"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/cbocovic/chord"
)

func TestNodeStart(t *testing.T) {
	ml := InitMLTransport()
	conf := DefaultConfig("test")
	conf.StabilizeMin = time.Duration(15 * time.Millisecond)
	conf.StabilizeMax = time.Duration(45 * time.Millisecond)
	r, err := Create(conf, ml)
	if err != nil {
		t.Fatalf("unexpected err. %s", err)
	}

	vn := makeVnode()
	vn.init()
	//set up flags
	addressPtr := flag.String("addr", "127.0.0.1:8888", "the port you will listen on for incomming messages")
	joinPtr := flag.String("join", "", "an address of a server in the Chord network to join to")

	flag.Parse()
	me := new(chord.ChordNode)

	//join node to network or start a new network
	if *joinPtr == "" {
		me = chord.Create(*addressPtr)
	} else {
		me = chord.Join(*addressPtr, *joinPtr)
	}
	fmt.Printf("My address is: %s.\n", *addressPtr)
	//block until receive input
Loop:
	for {
		var cmd string
		_, err := fmt.Scan(&cmd)
		switch {
		case cmd == "print":
			//print out successor and predecessor
			fmt.Printf("%s", me.Info())
		case cmd == "fingers":
			//print out finger table
			fmt.Printf("%s", me.ShowFingers())
		case cmd == "succ":
			//print out successor list
			fmt.Printf("%s", me.ShowSucc())
		case err == io.EOF:
			break Loop
		}
	}
	me.Finalize()
}

func prepRing(port int) (*Config, *TCPTransport, error) {
	listen := fmt.Sprintf("127.0.0.1:%d", port)
	conf := DefaultConfig(listen)
	conf.StabilizeMin = time.Duration(15 * time.Millisecond)
	conf.StabilizeMax = time.Duration(45 * time.Millisecond)
	timeout := time.Duration(20 * time.Millisecond)
	trans, err := InitTCPTransport(listen, timeout)
	if err != nil {
		return nil, nil, err
	}
	return conf, trans, nil
}

func TestTCPCreat(test *testing.T) {
	c, t, err := prepRing(10025)
	if err != nil {
		test.Fatalf("unexpected err. %s", err)
	}

	// Create initial ring
	r, err := Create(c, t)
	if err != nil {
		test.Fatalf("unexpected err. %s", err)
	}

	i := 0
	for {
		time.Sleep(20)
		fmt.Printf("Height = %d\n", i)
		i++
	}

	r.Shutdown()
	t.Shutdown()

}

func prepJoinRing(port int) (*Config, *TCPTransport, error) {
	listen := fmt.Sprintf("127.0.0.1:%d", port)
	conf := DefaultConfig(listen)
	conf.StabilizeMin = time.Duration(15 * time.Millisecond)
	conf.StabilizeMax = time.Duration(45 * time.Millisecond)
	timeout := time.Duration(20 * time.Millisecond)
	trans, err := InitTCPTransport(listen, timeout)
	if err != nil {
		return nil, nil, err
	}
	return conf, trans, nil
}

func TestNodeJoin(test *testing.T) {
	c, t, err := prepRing(10026)
	if err != nil {
		test.Fatalf("unexpected err. %s", err)
	}
	// Join ring
	r, err := Join(c, t, "127.0.0.1:10025")
	if err != nil {
		test.Fatalf("failed to join local node! Got %s", err)
	}

	t.Shutdown()
	r.Shutdown()
}
