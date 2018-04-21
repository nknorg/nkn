# Consensus and Blockchain

## Consensus Protocol

Consensus is achieved among nodes using the Major Vote Cellular
Automata (MVCA) algorithm as described in our whitepaper. MVCA is
efficient in both time and message count as only a few iterations of
communications between neighbors are needed to reach consensus, as
shown in the whitepaper.

Information required for consensus (e.g. next block) is sent to all
participating nodes at the beginning of the consensus through a
gossip-like protocol that takes up to O(logN) time, which is the main
time cost for consensus process. Such time can be reduced to
O(logN/logk) by having up to k times more neighbors similar to
Kademlia DHT as we may implement in our future version.

The consensus algorithm can be implemented by an event-driven protocol
triggered by consensus messages. Every consensus message has a topic
id identifying what is being agreed on. For each consensus message
received from a neighbor regarding a binary state (e.g. accept or
reject a block), the following procedures are executed:

1. If it’s the first message with the topic id, then compute self
initial state and send the state to neighbors.

2. Update sender’s state for the topic id.

3. If more than half of the nodes choosing self as neighbor have the
other state from self state for the topic id, then change self state
to the majority state and send the state to neighbors.

The consensus process for a topic id stops after a fixed amount of
time since first receiving messages with the same topic id. The
correctness and convergence of the MVCA algorithm is shown in our
whitepaper.

## Block Creation

New block is added to the blockchain by first selecting a “leader”
that proposes the new block. The leader selection is uncontrollable
but verifiable. The expected probability that a node is elected as
leader is proportional to the data relayed by the node on secure
paths. The selected leader proposes the next block and sends it to the
network for confirmation. The proposed block will be added to the
blockchain as the next block if consensus among nodes is reached.

### Leader Selection

Leader is selected by first selecting a signature chain on a secure
path. The signature chain being selected is the one that has the
lowest last signature value. As discussed before, signature cannot be
predicted or forged until a malicious party has controlled all nodes
in the chain. On the other hand, any malicious party has trivial
probability to control all nodes on a secure path. These two
properties combined guarantees that selecting the signature chain with
lowest last signature on secure paths is effectively randomly
selecting signature chain on secure paths and the selection cannot be
predicted or controlled by any party.

To select the signature chain with lowest last signature, each node
who signs the last signature checks if the last signature is smaller
than a threshold. If the last signature is smaller than the threshold,
the node sends out the signature chain to the network as a candidate
for the lowest last signature. The threshold is chosen so that with
high probability, the lowest last signature is smaller than the
threshold. Let t be the threshold in unit of largest possible
signature value, and the signature is distributed uniformly from 0 to
the largest value. The probability that all last signature are above
the threshold is (1 - t)^L, where L is the number of signature
chain. We need (1 - t)^L < e where e is small enough. For small t we
have t > - (log e) / L. We choose t = - (log e) / L to minimize the
number of candidates. L can be estimated from the average number of
signature chains in a number of previous blocks.

Nodes reach consensus about signature chain selection after receiving
candidates. The initial choice of each node is the signature chain
candidate with lowest last signature. If all nodes receive the same
candidates, then consensus is trivial as all honest nodes will make
the same initial choice. Otherwise the same signature chain (but not
necessarily the one with global lowest signature if initially it is
not received by enough nodes) will be guaranteed by the consensus
protocol.

When consensus about the signature chain is reached, leader is chosen
deterministic from the signature chain. Let S be the last signature of
the signature chain. Relay nodes on the signature chain are labeled
from 0 to L-1, where L is the number of relay nodes. Then the leader
is the node with label S mod L on the signature chain.

### Block Creation

Block is proposed by the selected leader in each round. Proposed block
is sent to all nodes for verification and consensus. During consensus
phase, each node sends to its neighbors the hash of the block (in case
the leader sends out different blocks to different neighbors) and if
it is accepted or not. When consensus is reached, all nodes should
have the same accepted block or none if rejected. Proposed block is
added to the blockchain if accepted.

## State Transition

Leader selection and block creation and running in parallel.

Leader selection for block epoch n starts after both leader selection
for block n-1 and block creation for block opoch n-2 are finished, and
runs in parallel with the block creation for block n-1. In leader
selection process, each node has 3 different states: listening state,
waiting for candidates state and candidate consensus state.

In listening state, node waits for signature chain that is valid for
mining (candidate). Once it receives or produces a valid candidate, it
transits into waiting for candidates state. In this state, node sends
out new candidate signature chain(s) it mined since last listening
state.

In waiting for candidates state, node still accepts and relays
candidates, but will not send out new candidates it mined. The propose
of this state is waiting for candidates sent out during listening
state to finish propagating. Node stays in waiting for candidates
state for a fixed time before transiting to candidate consensus state.

In candidate consensus state, node receives neighbors' states on each
candidate indicating whether to choose the candidate. If more than
half of the neighbors have the opposite state on a candidate than
itself, node updates its state and sends the new one to its
neighbors. Node also sends its initial states to neighbors when
entering candidate consensus state. Consensus is finished after a
fixed time since entering the candidate consensus state.

Block creation runs in parallel with leader selection, but one epoch
behind. Block creation for block epoch n starts after both leader
selection for block n and block creation for block n-1 are
finished. There are 2 states in block creation process: listening
state and block consensus state.

In listening state, node waits for block proposed by the selected
candidate. Once it receives a proposed block, it transits into block
consensus state.

Block consensus state is similar to candidate consensus state, except
that nodes vote for block rather than candidate. If node receives more
than one proposed block signed by the candidate, it will vote for none
of the proposed blocks because it indicates that the selected
candidate is not honest (propose multiple blocks).
