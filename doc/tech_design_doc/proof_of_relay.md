# Proof of Relay (PoR)

One of the key problems in NKN network is to prove how much data a
node relayed, which we define as Proof of Relay (PoR). PoR is crucial
to the NKN network as the amount of data being relayed is directly
related to token rewards. An ideal proof should satisfy the following
conditions: (1) verifiable: anyone, involved in data transmission or
not, is able to verify the proof correctly using only public
information; (2) unforgeable: no party, unless controlled all involved
nodes, is able to forge a valid proof with nontrivial probability; (3)
untamperable: no party, unless controlled all involved nodes, is able
to modify a valid proof with nontrivial probability.

We solve the problem using chain of signature. To improve efficiency,
we use the hash of the chain to randomly select proofs in an
uncontrollable but verifiable way. Moreover, we select a class of
proofs that is proven to be hard to forge even for a large malicious
party that has controlled a significant portion of all nodes in the
network to be eligible for other benefits such as mining rewards.

## Signature Chain as Proof of Relay

One of the primary functions of the NKN network is to transmit
data. To be transparent to higher level applications, NKN network
works with network packets. Packet originates from source which can be
either node or client, then relayed by relayers which can only be
nodes, and finally arrives at destination which can be either node or
client.

### NKN Packet

The basic transmission unit in the NKN network is NKN packet. A NKN
packet contains the following fields:

1. Payload (e.g. network packet)
2. Payload hash
3. Payload size
4. Source NKN address and public key
5. Destination NKN address and public key
6. Signature chain

### Signature Chain

A signature chain is a chain of signature, signed by data relayers
sequentially when relaying NKN packet. Each element of the chain
consists of the following fields:

1. Relayer NKN address and public key
2. Next relayer NKN address
3. Signature(signature of the previous element on chain, relayer NKN
address, next relayer NKN address) signed with relayer private key

The first element of the signature chain is signed by source, and the
signature field is replaced by signature(payload hash, payload size,
source NKN address and public key, destination NKN address and public
key, next relayer NKN address).

### Verifiability

A signature chain is valid if and only if for each element on chain,
the following conditions are satisfied:

1. The relayer NKN address matches the next relayer NKN address of the
previous element.
2. The signature field is valid.

Any modifications to a signed NKN packet except the payload will
invalidate the signature chain unless re-signed. Thus, a signature
chain is verifiable to anyone who has access to the NKN packet (no
need for the payload). A malicious party cannot tamper with or forge a
signature chain without having all the private keys along the route.

A NKN packet is valid if and only if the following conditions are
satisfied:

1. Payload hash is correct.
2. Payload size is correct.
3. Signature chain is valid.
4. Source NKN address and public key match the first element of the
signature chain.
5. Destination NKN address and public key match the last element of
the signature chain

Any modifications to a signed NKN packet will invalidate it unless
re-signed. Thus, a NKN packet is verifiable to anyone. No extra
information is needed to verify a NKN packet. A malicious party cannot
tamper with or forge a NKN packet without having all the private keys
along the route.

### Proof of Relay

We use NKN packet with payload field removed as a proof of relay work
for all relay nodes along the route. It satisfies our verifiable,
unforgeable and untamperable requirements such that everyone is able
to verify the validity of a signature chain, while no one can forge or
modify a valid signature chain without controlling (have private keys)
all nodes in the route.

Signature chain cannot be forked because each element contains the NKN
address and public key of the next node. If a node on the route is
malicious and removes or modifies some previous elements on the chain
when generating his signature, the chain is no longer
valid. Similarly, If a partially signed signature chain is intercepted
by a malicious party, no valid signature chain can be generated
without the private key of the designated next node.

## Efficient Proof of Relay

Every NKN packet has a proof which can be used as a receipt to
generate transactions from source to relayers. However, it is
inefficient to create a transaction for every packet transmitted in
the NKN network. Instead, only packets whose last signature on the
signature chain is smaller than a threshold are eligible for
transactions.

The last signature on the signature chain is verifiable to everyone,
while still being unpredictable and uncontrollable unless all nodes
along the route including source and destination are controlled by the
same party. The last signature is essentially deterministic given the
payload and the full path, but cannot be computed in advance without
all the private keys along the route.

With an ideal hash function, the last signature on the signature chain
is random across packets. Thus, selecting only packets with small
enough last signature for transactions (with adjusted price per
packet) does not change the expected rewards for relay nodes but
introduced some variation in pricing and rewards. The threshold should
be chosen to balance the need for less transactions and smaller reward
variation.

## Secure Proof of Relay from Malicious Party

Signature chain in NKN packet is unpredictable and uncontrollable as
long as the private key of at least one node is kept secret. However,
if a malicious party controls all nodes on the route including source
and destination, the party is able to predict and create valid
signature chain and NKN packet without actually transmitting the
packet. To solve this problem, we differentiate two classes of
signature chain. One class, which is mathematically shown to have
trivial probability to be controlled by a party, can be used to gain
more benefits such as mining rewards. The other class has no
additional benefits so that nothing but economic loss can be gained by
creating signature chains.

### Secure Path

We define a path from source to destination as secure path if the
following conditions are satisfied:

1. Each hop of the path is between designated neighbors.
2. The path is the designated path following the routing rule.
3. The length of the path is higher than a threshold.

Secure paths have vanishing probability to be controlled by the same
party as long as the party does not control a large enough portion of
all nodes in the NKN network, as shown in the following proof.

Consider a malicious party that controls M nodes in the NKN network
with N nodes in total. Since there is only one designated path between
any node pair, the number of designated path in the network is no more
than N^2. On the hand, the probability that all relayers on the secure
path between a malicious source/destination pair belong to the
malicious party cannot be greater than (M/N)^k, where k is the minimal
number of nodes in a secure path, and we assume the probability that
each relayer belong to the malicious party is independent as the NKN
address of node is mostly uncontrollable. Thus, the expected number of
secure path where all nodes belong to the malicious party is no more
than N^2 * (M/N)^k. In order for the malicious party to have trivial
probability to control any secure path, we need N^2 * (M/N)^k < e,
where e is a small number representing the maximal expected number of
secure path under malicious party’s control that we can tolerate. This
leads us to M/N < 2^(-(2 log_2(N) - log_2(e)) / k). When N is large we
have M/N < 2^(-2 log_2(N) / k). This essentially tells us that if the
fraction of nodes under maliciou party’s control is no more than 2^(-2
log_2(N) / k), then with high probability no secure path can be
controlled by the malicious party. If the path length threshold k is
close to the maximal expected path length log_2(N), then the malicious
party must control more than 1/4 of the nodes in order to be able to
control at least one secure path.

Therefore, signature chain with a secure path is unpredictable and
uncontrollable to any party that does not control a significant
portion of all nodes in the NKN network. It is reasonable to count the
relay work on a secure path towards more benefits such as mining
rewards.

### Other Paths

Paths other than secure path have higher chances to be controlled by
the same party. There are generally two types of path belonging to
this class: short designated path, and non-designated path. For short
designated path, the expected number of path fully controlled by a
large malicious party is nontrivial. For non-designated path, nodes
have the freedom to choose path, and a malicious party can easily
choose paths fully under its control. In both cases, there is a
nontrivial chance that a large malicious party could forge signature
chain on paths that only contain malicious nodes without actually
transmitting data.

To solve this problem, we do not count relay work on such paths
towards any other benefits (e.g. mining rewards). The only rewards
that relay nodes get for such relay path is the token paid by
source. Since the sum of rewards that all relayers get is no more than
the amount paid by source, malicious party does not gain anything but
economic loss (transaction fees) by forging signature chain.

## Session based Transmission

For heavy network traffic, establishing a signature chain for each
packet is inefficient. Here we introduce another concept called
session. When users want to communicate with another NKN node, they
can establish a session to avoid signing every packet.

We have two different versions of this protocol, a deterministic
version and a probabilistic version. We implement the deterministic
version in the early stage as it is simpler. If there are tens of
thousands users in NKN, the throughput of blockchain becomes a
bottleneck. We are designing a probabilistic version to reduce the
data on the blockchain. The two different versions share many similar
ideas, so we introduce two of them simultaneously.

### Establish Session

In order to establish a session, the participants need to agree on the
following things:

* The owner of the session. (Who will pay for it.)
* Timestamp chosen by the owner. 
* A relayer list specifying the path of session. (The last element is
  called destination)
* Each relayer element should contain the following part
  * Public Key
  * Price
  * Hash Anchor (Only required in probabilistic version)

These part is called the metadata of this session. Making consensus of
these metadata may be based on signature chain and some other market
matching and bargaining mechanism. We skip the details here.

Once metadata is generated, we construct a signature chain for this
metadata. The hash value of metadata and signature is session ID.

### Data Transmission

Session based transmission can reduce the average overhead of each
packet significantly. Each packet only needs to contains a header
includes the following:

* Session ID
* Direction (Owner → Destination or Destination → Owner)
* Nonce (The nonce of two direction are count respectively.)

Note: the term “packet” here is not the IP packet in network layer, it
is a basic billing unit in data transmission. The sender can determine
how to divide all transmission data into packets. For example, a GET
request, a response of HTTP request. Each packet should not be too
large. (For example, < 1MB)
