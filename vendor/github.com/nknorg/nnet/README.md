# nnet: a fast, scalable, and developer-friendly p2p network stack in Go

## Introduction

**nnet** is designed as a fast, scalable, and developer-friendly network stack/layer
for decentralized/distributed systems. It handles message delivery, topology
maintenance, etc in a highly efficient and scalable way, while providing enough
control and flexibility through developer-friendly messaging interface and powerful
middleware architecture.

## Features

* nnet uses a **modular and layered overlay architecture**. By default an improved and much more reliable version of Chord DHT protocol is used to maintain a scalable overlay topology, while other topologies like Kademlia can be easily added by implementing a few overlay interfaces.
* Extremely easy to use message sending interface with **both async and sync message flow** (block until reply). Message reply in sync mode are handled efficiently and automatically, you just need to provide the content.
* Deliver message to any node in the network (**not just the nodes you are directly connected to**) reliably and efficiently in at most log_2(N) hops (w.h.p) where N is the total number of nodes in the network.
* **Novel and highly efficient** message broadcasting algorithm with exact once (or K-times where K is something adjustable) message delivery that achieves **optimal throughput and near-optimal latency**. This is done by sending message through the spanning tree constructed by utilizing the Chord topology.
* **Powerful and flexible middleware architecture** that allows you to easily interact with node/network lifecycle and events like topology change, routing, message sending and delivery, etc. **Applying a middleware is as simple as providing a function**.
* Flexible **transport-aware address scheme**. Each node can choose its own transport protocol to listen to, and nodes with different transport protocol can communicate with each other transparently. nnet supports TCP and KCP transport layer by default. Other transport layers can be easily supported by implementing a few interfaces.
* **Modular and extensible router architecture**. Implementing a new routing algorithm is as simple as adding a router that implements a few router interfaces.
* Only **a fixed number of goroutines** will be created given network size, and the number can be changed easily by changing the number of concurrent workers.
* **NAT traversal** (UPnP and NAT-PMP) using middleware.
* Use protocol buffers for message serialization/deserialization to support cross platform and backward/forward compatibility.
* Provide your own logger for logging integration with your application.

Coming soon:

- [] Latency measurement
- [] Proximity routing
- [] Push-pull broadcasting for large messages
- [] Efficient Pub/Sub
- [] Test cases

## Getting Started

### Requirements:

* Go 1.10+

### Install

```bash
go get -u -d github.com/nknorg/nnet
```

### Basic

The bare minimal way to create a node is simply

```go
nn, err := nnet.NewNNet(nil, nil)
```

This will create a nnet node with random ID and default configuration (listen to
a random port). Starting the node is as simple as

```go
err = nn.Start()
```

To join an existing network, simply call `Join` after node has been started

```go
err = nn.Join("<protocol>://<ip>:<port>")
```

Put them together, a local nnet cluster with 10 nodes can be created by just a
few lines of code.

```go
nnets := make([]*nnet.NNet, 10)
for i := 0; i < len(nnets); i++ {
  nnets[i], _ = nnet.NewNNet(nil, nil) // error is omitted here for simplicity
  nnets[i].Start() // error is omitted here for simplicity
  if i > 0 {
    nnets[i].Join(nnets[0].GetLocalNode().Addr) // error is omitted here for simplicity
  }
}
```
