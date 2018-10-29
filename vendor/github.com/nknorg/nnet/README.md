# nnet: a fast and scalable p2p network layer in Go

## Introduction

nnet is designed as the network layer of decentralized/distributed systems. It
handles message delivery, topology maintenance, etc in a highly efficient and
scalable way, while providing enough control and flexibility through middleware
architecture.

## Features

* nnet uses an improved and much more reliable version of Chord DHT protocol to maintain an overlay topology by default, while other topologies like Kademlia can easily be added by implementing a few interfaces.
* Deliver message to any node in the system (not just the nodes you are directly connected to) reliably and efficiently in at most log_2(N) hops (w.h.p) where N is the total number of nodes in the network.
* Highly efficient message broadcasting with exact once message delivery that achieves optimal throughput and near-optimal latency.
* Only a fixed number of goroutines will be created given network size, and can be changed easily by changing the number of concurrent workers.
* NAT traversal (UPnP and NAT-PMP) using middleware.
