# NKN: a Scalable Self-Evolving and Self-Incentivized Decentralized Network

> A project aiming to rebuild the Internet that will be truly open,
  decentralized, dynamic, safe, shared and owned by the community.

                                                               [![nkn.org](img/nkn_logo.png)](https://nkn.org/)

[https://nkn.org/](https://nkn.org/)

Welcome to NKN! Watch a [2-minutes video of NKN
concept](https://youtu.be/cT29i3-ImQk) to get started.

[![NKN Introduction](img/nkn_intro_video.png)](https://youtu.be/cT29i3-ImQk)

## Table of Contents

- [Abstract](#abstract)
- [Challenges of Network Infrastructure](#challenges-of-network-infrastructure)
- [Vision](#vision)
- [Technology Foundations](#technology-foundations)
- [New Kind of Network](#new-kind-of-network)
- [Cellular Automata Powered Consensus](#cellular-automata-consensus)
- [Potential Attacks](#potential-attacks)
- [Conclusions](#conclusions)


## Abstract

NKN is a new generation of highly scalable, self-evolving and
self-incentivized blockchain network infrastructure. NKN addresses the
network decentralization and self-evolution by introducing Cellular
Automata (CA) methodology. NKN tokenizes network connectivity and data
transmission capacity by a novel and useful Proof of Work. NKN focuses
on decentralizing network resources, similar to how Bitcoin and
Ethereum decentralize computing power as well as how IPFS and Filecoin
decentralize storage. Together, they form the three pillars of the
Internet infrastructure for next generation blockchain systems. NKN
ultimately makes the network more decentralized, efficient, equalized,
robust and secure, thus enabling healthier, safer, and more open
Internet.


## Challenges of Network Infrastructure

After years of transmutation, the Internet is in danger of losing its
original vision and spirit.  Existingsolutions are not suitable for next
 generation blockchain systems due to the following reasons:

A highly reliable, secure and diverse Internet is essential to
everyone. Yet, huge inefficiency exists in the current network when
providing global connectivity and information transmission. It's time
to rebuild the network we want, not just patch the network we have. A
fully decentralized and anonymous peer-to-peer system offers huge
potential in terms of improved efficiency, sustainability and safety
for industry and society.

When the Federal Communications Commission (FCC) approved
 a measure to remove the net neutrality rules by the end of 2017, a demand 
 of ending our reliance on big telecommunication monopolies and building
decentralized, affordable, locally owned Internet infrastructure becomes ever 
stronger. The unrestricted and private Internet access environment is becoming 
unsustainable under an endless stream of attacks and blockage, leading to selective
 and biased information propagation. Without a proper incentivizing engagement 
 scheme, it is almost impossible to maintain a constant and secured information
propagation channel.

## Vision

NKN intends to revolutionize the entire network technology and business. NKN wants
 to be the Uber or Airbnb of the trillion-dollar communication service business, but 
 without a central entity. NKN aspires to free the bits, and build the Internet we 
 always wanted. By blockchainizing the third and probably the last pillar of 
 Internet infrastructure,  NKN will revolutionize the blockchain ecosystem by innovating
 on the network layer, after Bitcoin and Ethereum blockchainized computing power as 
well as IPFS and Filecoin blockchainized storage. The next generation blockchains based
 on NKN are capable of supporting new kind of decentralized applications (DApp)
 which have much more powerful connectivity and transmission capability. The vision of
 NKN is not only to revolutionize the decentralized network layers, but also to develop
core technologies for the next generation blockchain. More details can be found in [Overview]
 (https://github.com/nknorg/nkn/blob/master/doc/tech_design_doc/overview.md).

## Technology Foundations

NKN utilizes microscopic rules based on Cellular Automata (CA) to define
network topology, achieves self-evolution behaviors, and explores
Cellular Automata driven consensus, which is fundamentally different
from existing blockchain network layer.

CA is a state machine with a collection of nodes, each changing its state
 following a local rule that only depends on its neighbors. Each node only
 has a few neighbor nodes. Propagating through local interactions, local states 
 will eventually affect the global behavior of CA. The desired openness of 
 network is determined by the homogeneity of Cellular Automata where all nodes
 are identical, forming a fully decentralized P2P (peer-to-peer) network. Each
 node in the NKN network is constantly updating based on its current state as
well as the states of neighbors. The neighbors of each node are also dynamically 
changing so that the network topology is also dynamic without changing its 
underlying infrastructure and protocols.

The Cellular Automata programming formula is called "local rule",
which is an indispensable rule for next generation network of NKN and
has an important influence on the network topology. Proper choice of
local rules leads to Cellular Automata with complex but self-organized
behaviors on the boundary between stability and chaos. Rules are
essential because they are formulas to program Cellular Automata and
Automata Networks.


## New Kind of Network

NKN introduced the concept of Decentralized Data Transmission Network
(DDTN). DDTN combines multiple independent and self-organized relay
nodes to provide clients with connectivity and data transmission
capability. This coordination is decentralized and does not require
trust of any involved parties. The secure operation of NKN is achieved
through a consensus mechanism that coordinates and validates the
operations performed by each node. DDTN provides a variety of
strategies for decentralized application (DApp). In contrast to centralized 
network connectivity and data transmission, there are multiple efficient
paths between nodes in DDTN, which can be used to enhance data transmission
capacity. Native tokens can incentivize the sharing of network resources, and
eventually minimize wasted connectivity and bandwidth. Such a property is
termed "self-incentivized". More details can be found in [DDTN]
 (https://github.com/nknorg/nkn/blob/master/doc/tech_design_doc/distributed_data_transmission_network.md).

## Cellular Automata Powered Consensus

NKN is designed to be a futuristic blockchain infrastructure that
requires low latency, high bandwidth, extremely high scalability and
low cost to reach consensus. These properties are crucial for future
DApps. Thus, NKN needs new consensus algorithms that could satisfy
such high requirements.

Majority Vote Cellular Automata (MVCA) is a Cellular Automata using
majority vote as updating rule to achieve consensus.Using the mathematical
 framework originally developed for Ising model in physics, we found and 
 proved that a class of CA rules will guarantee to reach consensus in at most
O(N) iterations using only states of sparse neighbors by an exact map from 
CA to zero temperature Ising model. Some studies investigated the fault 
 tolerance of Cellular Automata and how to increase robustness in Cellular 
Automata-Based systems. We further showed that the result is robust to random
 and malicious faulty nodes and compute the threshold when desired consensus
 cannot be made. More details can be found in [Ising Consensus and Blockchain]
 (https://github.com/nknorg/nkn/blob/master/doc/tech_design_doc/consensus_and_blockchain.md).

Consensus in NKN is driven by Proof of Relay (PoR), a useful Proof of
Work (PoW) where the expected rewards a node gets depend on its
network connectivity and data transmission power. Node proves its
relay workload by adding digital signature when forwarding data, which
is then accepted by the system through consensus algorithm. PoR is not a 
waste of resources since the work performed in PoR benefits the whole network
 by providing more transmission power. The "mining" is redefined as contributing
 to the data transmission layer, and the only way to get more rewards is providing 
 more transmission power. The competition between nodes in the network will eventually
drive the system towards the direction of low latency, high bandwidth data transmission network.
More details can be found in [Proof of Relay]
 (https://github.com/nknorg/nkn/blob/master/doc/tech_design_doc/proof_of_relay.md).

## Potential Attacks

Since NKN is designed with attack prevention in mind, it is necessary
to review related attack types. Attack analysis and mitigation will be
one of the important aspects of NKN development and future work.
The attackt types under study are collusion attacks, double spending 
attacks, Sybil attacks, denial-of-service (DoS) attacks, quality-of-service
 (QoS) attacks, eclipse attacks, selfish mining attacks, and fraud attacks.
 More details can be found in [Attack Analysis]
 (https://github.com/nknorg/nkn/blob/master/doc/tech_design_doc/attack_analysis.md).

## Conclusions

Compare to current systems, NKN blockchain platform is more suitable
for peer-to-peer data transmission and connectivity. In the meantime,
this self-incentivized model encourages more nodes to join the
network, build a flat network structure, implement multi-path routing,
and create a new generation of network transmission structure.

From the perspectives of computing infrastructure innovation, NKN will
revolutionize the blockchain ecosystem by blockchainizing the third
and probably the last pillar of Internet infrastructure, after Bitcoin
and Ethereum blockchainized computing power as well as IPFS and
Filecoin blockchainized storage. Complementing the other two pillars
of the blockchain revolution, NKN will be the next generation
decentralized network that is self-evolving, self-incentivized, and
highly scalable.

The current network has huge inefficiencies for providing universal
connectivity and access for all information and applications. It's
time to rebuild the network we really need instead of constantly
patching the networks we already own. Let's start building the future
Internet, today.