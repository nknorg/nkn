# Distributed Data Transmission Network (DDTN)

As a data transmission network, NKN network may contain millions of
nodes or more. Moreover, the network is dynamic as every node could
join or leave the network at any time. At such scale, it is
unrealistic for every node to maintain an up-to-date list of all nodes
in the network. Instead, every node in the network is only connected
to and aware of a few other nodes in the network which are called
neighbors.

Network topology, determined by the choice of neighbors, is crucial to
performance and security. To be scalable and secure, we need to choose
a proper topology that has the following properties: (1) network
should be connected and has small diameter; (2) efficient routing
algorithms exist between any node pairs using only information about
neighbors; (3) load is balanced among nodes given random traffic with
uniformly distributed source and destination; (4) choice of neighbors
should be unpredictable but verifiable to prevent attacks.

We use the network topology similar to Chord Distributed Hash Table
(DHT). Each node has an m-bit random address on the ring. Node is
connected to a set of other O(logN) nodes that have specific distance
from it such that the choice of neighbors can be verified. Routing
between any node pair is up to O(logN) and is deterministic and
verifiable given the topology.

There are two major types of devices in NKN network: nodes and
clients. A node is a device that sends, receives and relays data. A
client is a device that only sends and receives data, but not relays
data. Clients interact with the rest of the NKN network through nodes.

Nodes are the maintainers and contributors of the NKN network and thus
receive token rewards. A node needs to have a public IP address to
receive messages from nodes that have not yet established connections
with it.

Clients are consumers in the NKN network that pay nodes to relay data
or receive data through nodes. A client does not need to have an
public IP address as it will establish connections with nodes and then
send and receive data through them.

## NKN Address

Every node and client in NKN network has a unique m-bit NKN address
between 0 and 2^m - 1. Nodes and clients share the same address
space. Once a node or client joins the NKN network and has a NKN
address, IP address is no longer needed for other nodes to send
traffic to it. Instead, NKN address is the only information needed to
send packets to a node or client. The role of NKN address in NKN
network is similar to IP address in the Internet.

To guarantee the randomness of NKN address and prevent malicious nodes
from choosing specific NKN address, an unpredictable, uncontrollable
yet still verifiable function is used to generate NKN address when a
node joins NKN network. We choose the hash of public IP address and
latest block hash at the time of node joining

Node NKN Address = hash(node public IP, latest block hash)

where we require every node (but not client) has public IP address.

Client chooses NKN address similarly, but a random nonce chosen by
client is also added to the hash function to prevent collision when
multiple clients having the same public IP address join the NKN
network within the same block epoch

Client NKN Address = hash(client public IP, latest block hash, random nonce)

It is important that NKN address is random (or at least uniformly
distributed and uncontrollable) for two reasons. First, it is hard for
malicious nodes to choose specific NKN addresses. Being able to choose
NKN address makes it easier for malicious nodes to attack the system
as neighbors and routing are based on NKN addresses. Second, load is
more balanced among nodes given random NKN addresses, effectively
making the network more decentralized.

## Distance Metric

The distance between two NKN addresses x and y is defined as

dist(x, y) = min((x - y) mod 2^m, (y - x) mod 2^m)

which satisfies the following conditions

* dist(x, y) >= 0

* dist(x, y) = 0 if and only if x = y

* dist(x, y) = dist(y, x)

* dist(x, z) <= dist(x, y) + dist(y, z)

* dist(x, y) = dist(x + a, y + a)

Unlike Chord DHT, our distance metric is symmetric as we allow
bidirectional routing.

## Network Topology

We first define the successor and predecessor node of any NKN address x

successor(x) = argmin_y {(y - x) mod 2^m | y in all nodes}

predecessor(x) = argmin_y {(x - y) mod 2^m | y in all nodes}

Successor and predecessor of a node can be defined similarly, except
that the successor and predecessor of a node cannot be itself. If node
x is the successor of node y, then y is the predecessor of x.

Choice of neighbors is similar to Chord DHT but on both
directions. Each node x in NKN network has up to 2m neighbors:

successor((x + 2^i) mod 2^m), 0 <= i < m

predecessor((x - 2^i) mod 2^m), 0 <= i < m

Since not all NKN addresses are assigned to nodes, some neighbors
above point to the same node, in which case there are less than 2m
neighbors. It is worth mentioning that neighbors are generally
asymmetric such that if y is a neighbor of x, x may not be a neighbor
of y (except for the immediate neighbors, i.e. successor and
predecessor). Although the number of neighbors of a node A cannot be
more than 2m, the number of nodes that choose A as a neighbor can be
up to O(N) if the NKN addresses are highly unbalanced (while the
expected value is still up to O(m)).

Clients connect to the NKN network through nodes. Client at NKN
address x is connected to two nodes: successor(x) and
predecessor(x). The client sends and receives data only through
established connections with these two nodes and thus no client public
IP address is needed. Clients in the NKN network are similar to keys
in Chord DHT. Each client is connected to two nodes rather than one
node in Chord DHT to increase routing efficiency and fault tolerance.

Although there are other topology that may lead to more efficient
routing path, our choice has a critical advantage that is each choice
of neighbor can be verified by at least one other node. Suppose a node
at address x should choose A as its i-th successor node. In such case,
there are no nodes between address (x + 2^i) mod 2^m and A, and A
should have known this information from its predecessor. Thus A knows
that itself should be a neighbor of x. If node x does not choose A as
a neighbor, A is able to falsify it. Similarly, if x chooses another
node between A and (x + 2^(i+1)) mod 2^m in addition to A as
neighbors, A should also be able to falsify it. Verifiability is
crucial to an open network that may have a nontrivial portion of
malicious nodes. It greatly reduces the possibility that malicious
nodes choose specific neighbors to attack the system.

## Routing

Given an established NKN network, routing can be achieved in a greedy
algorithm using only information about neighbors. When a node wants to
send or relay a packet to a destination address, it sends the packet
to the neighbor that has the smallest distance to the destination
address. The remaining distance becomes at most the half of the
previous value for each relay, thus decreases exponentially or faster
as the packet is being relayed. Similar to Chord DHT, the expected
number of hops is O(logN) given uniformly distributed addresses.

If the destination address is a client rather than a node, the above
routing stops as long as the packet reaches any of the two nodes
connected to the client. The node then relays the packet to the client
through the established connection.

The route between any two node pair is verifiable. Suppose a node
sending a packet to destination address D should choose its neighbor A
as the next hop while it actually chose neighbor B. From the routing
rule we know that dist(A, D) < dist(B, D). Then A is able to falsify
the route. If the node chose a non-neighbor node as the next hop, then
such route can be falsified by the actual neighbor as discussed
previously. The verifiability of routing is crucial to reduce the
possibility of forming routes consisting of only malicious nodes.

## Protocols

NKN network is an open network. It is inevitable that nodes will join
and leave the network at any time, with or without prior
notification. To efficiently maintain the NKN network, each node
locally maintains three lists

1. Its neighbors and their successors and predecessors. Successors and
predecessors of neighbors are not required but will help recover
topology quickly from failed nodes.

2. Nodes that choose it as neighbors. This is not required but will make
updates more efficient.

3. Clients connected to it.

Nodes updates these lists in response to topology changes. For
efficiency, it is important that when a node or client join or leave
the network, only a small portion of the other nodes (up to O(logN))
are updates. We have the following protocols that satisfy our goal in
dynamic environments.

### Node Join

When a node wants to join the NKN network, it needs to know at least
one other node that is already in the network. We provide bootstrap
servers for this purpose while other sources can also be used. After
getting in touch with at least one other node in the NKN network, it
executes the following procedures:

1. Ask for the latest block hash to compute its own NKN address x.

2. Send messages to successor(x) and predecessor(x) to get its successor
and predecessor.

3. Ask its successor and predecessor for their neighbors, nodes that
choose them as neighbors, and clients that are connected to them.

4. Identify neighbors using the successor neighbors of its predecessor
and predecessor neighbors of its successor. For neighbors that cannot
be determined, send message to the theoretical NKN address to get
actual neighbor address.

5. Identify nodes that need to update their neighbors using the list
of nodes that choose its successor and predecessor as
neighbors. Notify these nodes to update neighbors.

6. Take over responsive clients from its successor and predecessor.

### Node Leave

When a node wants to leave the NKN network, it executes the following
procedures:

1. Notify nodes that choose it as neighbors to update their neighbors
by sending them its successor or predecessor.

2. Notify its neighbors that it will no longer choose them as
neighbors.

3. Hand its responsive clients to its successor or predecessor.

In case of node fails (leave without executing the above procedures),
other nodes choose it as neighbors will notice it through the
periodical keepalive messages and update neighbors using the successor
and predecessor of neighbors. Similarly, its neighbors will also
notice this and remove it from the list. Clients connected to it will
only use the other connected node for gateway before connected to a
new node.

### Client Join

When a client wants to join the network, it first gets in touch with
at least one node in the network, and executes the following
procedures:

1. Ask for the latest block hash to compute its own NKN address x.

2. Send messages to successor(x) and predecessor(x) to get its two
connected nodes and establish connections

### Client Leave

When a client wants to leave the network, it notifies its connected
nodes to remove it from the client list.

In case of client fails, its connected nodes will notice this as the
established connections fail. Nodes then remove the client from the
list.

### Keepalive Message

Each node periodically sends keepalive messages to its neighbors. When
a node receives a keepalive message, it sends back its successor and
predecessor in response, which is used to maintain the successor and
predecessor of neighbors at each node.
