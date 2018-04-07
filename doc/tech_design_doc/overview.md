# Overview

The core of the NKN network are many connected nodes. Every node is
only connected to and aware of a few other nodes called neighbors. The
topology of the network is verifiable. Data can be transmitted from
any node to any other node in an efficient and verifiable route. The
network is dynamic such that any node can join and leave the network
at any time without causing system failures. Data can also be sent
from and to client (which only sends and receives data but does not
relay data or participate in consensus) without public IP address.

The relay workload can be verified using our Proof of Relay (PoR)
algorithm. A small and fixed portion of the packets will be randomly
selected as proof. The random selection can be verified and cannot be
predicted or controlled. Proof will be sent to other nodes for payment
and rewards.

A node in our network is both relayer and consensus
participant. Consensus among massive nodes can be reached efficiently
by only communicating with neighbors using our algorithm. Consensus is
reached for every block to prevent fork.
