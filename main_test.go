package main

import (
	"fmt"
	"time"
	"nkn-core/net/chord"
)

// func prepRing(port int) (*chord.Config, *chord.TCPTransport, error) {
// 	listen := fmt.Sprintf("localhost:%d", port)
// 	conf := chord.DefaultConfig(listen)
// 	conf.StabilizeMin = time.Duration(15 * time.Millisecond)
// 	conf.StabilizeMax = time.Duration(45 * time.Millisecond)
// 	timeout := time.Duration(20 * time.Millisecond)
// 	trans, err := chord.InitTCPTransport(listen, timeout)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	return conf, trans, nil
// }

// // Creat the ring
// func main() {
// 	c, t, err := prepRing(10025)
// 	if err != nil {
// 		fmt.Printf("unexpected err. %s", err)
// 	}

// 	// Create initial ring
// 	r, err := chord.Create(c, t)
// 	if err != nil {
// 		fmt.Printf("unexpected err. %s", err)
// 	}

// 	i := 0
// 	for {
// 		time.Sleep(20 * time.Second)
// 		fmt.Printf("Height = %d\n", i)
// 		i++
// 	}

// 	r.Shutdown()
// 	t.Shutdown()
// }

func prepJoinRing(port int) (*chord.Config, *chord.TCPTransport, error) {
	listen := fmt.Sprintf("localhost:%d", port)
	conf := chord.DefaultConfig(listen)
	conf.StabilizeMin = time.Duration(15 * time.Millisecond)
	conf.StabilizeMax = time.Duration(45 * time.Millisecond)
	timeout := time.Duration(20 * time.Millisecond)
	trans, err := chord.InitTCPTransport(listen, timeout)
	if err != nil {
		return nil, nil, err
	}
	return conf, trans, nil
}

// Join the ring
func main() {
	c, t, err := prepJoinRing(10026)
	if err != nil {
		fmt.Printf("unexpected err. %s", err)
	}
	// Join ring
	r, err := chord.Join(c, t, "localhost:10025")
	if err != nil {
		fmt.Printf("failed to join local node! Got %s", err)
	}

	time.Sleep(20 * time.Second)

	t.Shutdown()
	r.Shutdown()
}
