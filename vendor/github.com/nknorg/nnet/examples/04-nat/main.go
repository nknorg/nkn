// This example shows how to setup NAT traversal. The example uses go-nat
// package, which only works for routers that support UPnP and NAT-PMP protocol.
// NAT traversal can also be set up in middlewares such as NetworkWillStart and
// LocalNodeWillStart.

// You may need to get go-nat package by `go get github.com/nknorg/go-nat` in
// order to run this example.

// Run with default options: go run main.go

// Show usage: go run main.go -h

package main

import (
	"flag"
	"os"
	"os/signal"
	"time"

	gonat "github.com/nknorg/go-nat"
	"github.com/nknorg/nnet"
	"github.com/nknorg/nnet/log"
	"github.com/nknorg/nnet/transport"
)

func main() {
	transportPtr := flag.String("t", "tcp", "transport type, tcp or kcp")
	externalPortPtr := flag.Uint("e", 10086, "external port to map from")
	internalPortPtr := flag.Uint("i", 12580, "internal port to map to")
	flag.Parse()

	conf := &nnet.Config{
		Port:      uint16(*externalPortPtr),
		Transport: *transportPtr,
	}

	nn, err := nnet.NewNNet(nil, conf)
	if err != nil {
		log.Error(err)
		return
	}

	transport, err := transport.NewTransport(*transportPtr)
	if err != nil {
		log.Error(err)
		return
	}
	transportProtocol := transport.GetNetwork()

	// Begin of NAT setup
	// This can also be done in middleware like NetworkWillStart or LocalNodeWillStart
	// ==========================================================================
	log.Info("Discovering NAT gateway...")

	nat, err := gonat.DiscoverGateway()
	if err != nil {
		log.Error(err)
		return
	}

	log.Infof("Found %s gateway", nat.Type())

	externalPort, internalPort, err := nat.AddPortMapping(transportProtocol, int(*externalPortPtr), int(*internalPortPtr), "nnet", 10*time.Second)
	if err != nil {
		log.Error(err)
		return
	}

	defer nat.DeletePortMapping(transportProtocol, int(*externalPortPtr))

	log.Infof("Mapped external port %d to internal port %d", externalPort, internalPort)

	// SetInternalPort should only be called before node starts
	nn.GetLocalNode().SetInternalPort(uint16(internalPort))
	// ==========================================================================
	// End of NAT setup

	err = nn.Start()
	if err != nil {
		log.Error(err)
		return
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	log.Info("\nReceived an interrupt, stopping...\n")

	nn.Stop(nil)
}
