# NKN: a Scalable Self-Evolving and Self-Incentivized Decentralized Network

> A project aiming to rebuild the Internet that will be truly open,
  decentralized, dynamic, safe, shared and owned by the community.

[![nkn.org](img/nkn_logo.png)](https://nkn.org/)

[https://nkn.org/](https://nkn.org/)

Welcome to NKN! Watch a [2-minutes video of NKN
concept](https://youtu.be/cT29i3-ImQk) to get started.

[![NKN Introduction](img/nkn_intro_video.png)](https://youtu.be/cT29i3-ImQk)

## Table of Contents

- [Challenges of Network Infrastructure](#challenges-of-network-infrastructure)
- [Vision](#vision)
- [Technology Foundations](#technology-foundations)
- [New Kind of Network](#new-kind-of-network)
- [Cellular Automata Powered Consensus](#cellular-automata-consensus)
- [Potential Attacks](#potential-attacks)
- [Building](#building)
- [Contributing](#contributing)
- [Community](#community)
- [Conclusions](#conclusions)

## Challenges of Network Infrastructure

After years of transmutation, the Internet is in danger of losing its
original vision and spirit.  When the FCC approved a measure to remove the
 net neutrality rules by the end of 2017, a demand of ending our reliance on 
big telecommunication monopolies and building
decentralized, affordable, locally owned Internet infrastructure becomes ever 
stronger. The unrestricted and private Internet access environment is becoming 
unsustainable under an endless stream of attacks and blockage, leading to 
selective and biased information propagation. Without a proper incentivizing 
engagement scheme, it is almost impossible to maintain a constant and secured
 information propagation channel.

## Vision

NKN intends to revolutionize the entire network technology and business. NKN
 wants  to be the Uber or Airbnb of the trillion-dollar communication service
business, but without a central entity. NKN aspires to free the bits, and build
the Internet we always wanted. By blockchainizing the third and probably the
last pillar of Internet infrastructure,  NKN will revolutionize the blockchain
ecosystem by innovating on the network layer, after Bitcoin and Ethereum 
blockchainized computing power as well as IPFS and Filecoin blockchainized
storage. The vision of NKN is not only to revolutionize the decentralized
network layers, but also to develop core technologies for the next generation
blockchain. More details can be found in [Overview](doc/tech_design_doc/overview.md).

## Technology Foundations

NKN utilizes microscopic rules based on Cellular Automata (CA) to define
network topology, achieves self-evolution behaviors, and explores
Cellular Automata driven consensus, which is fundamentally different
from existing blockchain network layer.CA is a state machine with a collection 
of nodes, each changing its state following a local rule that only depends on its
neighbors. Each node only has a few neighbor nodes. Propagating through local
interactions, local states will eventually affect the global behavior of CA. 
The desired openness of network is determined by the homogeneity of Cellular 
Automata where all nodes are identical, forming a fully decentralized P2P
network. Each node in the NKN network is constantly updating based on its
current state as well as the states of neighbors. The neighbors of each node are
also dynamically changing so that the network topology is also dynamic without
changing its underlying infrastructure and protocols.

## New Kind of Network

NKN introduced the concept of Decentralized Data Transmission Network
(DDTN). DDTN combines multiple independent and self-organized relay
nodes to provide clients with connectivity and data transmission capability.
DDTN provides a variety of strategies for decentralized application (DApp).
In contrast to centralized network connectivity and data transmission, there are 
multiple efficient paths between nodes in DDTN, which can be used to enhance 
data transmission capacity. Native tokens can incentivize the sharing of network 
resources, and eventually minimize wasted connectivity and bandwidth. Such 
a property is termed "self-incentivized". More details can be found
in [DDTN](doc/tech_design_doc/distributed_data_transmission_network.md).

## Cellular Automata Powered Consensus

Majority Vote Cellular Automata (MVCA) is a Cellular Automata using
majority vote as updating rule to achieve consensus.Using the mathematical
 framework originally developed for Ising model in physics, we found and 
 proved that a class of CA rules will guarantee to reach consensus in at most
O(N) iterations using only states of sparse neighbors by an exact map from 
CA to zero temperature Ising model. More details can be found in
[Ising Consensus and Blockchain](doc/tech_design_doc/consensus_and_blockchain.md).

Consensus in NKN is driven by Proof of Relay (PoR), a useful Proof of
Work (PoW) where the expected rewards a node gets depend on its
network connectivity and data transmission power. PoR is not a 
waste of resources since the work performed in PoR benefits the whole network
 by providing more transmission power. The "mining" is redefined as contributing
 to the data transmission layer, and the only way to get more rewards is providing 
 more transmission power. The competition between nodes in the network will eventually
drive the system towards the direction of low latency, high bandwidth data transmission network.
More details can be found in [Proof of Relay](doc/tech_design_doc/proof_of_relay.md).

## Potential Attacks

Since NKN is designed with attack prevention in mind, it is necessary
to review related attack types. Attack analysis and mitigation will be
one of the important aspects of NKN development and future work.
The attackt types under study are collusion attacks, double spending 
attacks, Sybil attacks, denial-of-service (DoS) attacks, quality-of-service
 (QoS) attacks, eclipse attacks, selfish mining attacks, and fraud attacks.
 More details can be found in [Attack Analysis](doc/tech_design_doc/attack_analysis.md).
 
## Building
 
## Contributing
 
## Community

## Conclusions

NKN will revolutionize the blockchain ecosystem by blockchainizing
the third and probably the last pillar of Internet infrastructure, after Bitcoin
and Ethereum blockchainized computing power as well as IPFS and
Filecoin blockchainized storage. Complementing the other two pillars
of the blockchain revolution, NKN will be the next generation decentralized
network that is self-evolving, self-incentivized, and highly scalable.

The current network has huge inefficiencies for providing universal
connectivity and access for all information and applications. It's time to
rebuild the network we really need instead of constantly patching the networks
we already own. Let's start building the future Internet, today.