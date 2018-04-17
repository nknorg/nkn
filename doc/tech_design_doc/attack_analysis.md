# Attack Analysis

- [Collusion Attacks](#collusion-attacks)
- [Sybil Attacks](#sybil-attacks)
- [Double Spending Attacks](#double-spending-attacks)
- [Denial of Service Attacks](#denial-of-service-attacks)
- [Quality of Service Attacks](#quality-of-service-attacks)
- [Eclipse Attacks](#eclipse-attacks)
- [Selfish Mining Attacks](#selfish-mining-attacks)
- [Fraud Attacks](#fraud-attacks)

## Collusion Attacks

Collusion attacks are performed by malicious nodes who share their knowledge to cheat for rewards . Colluding nodes know the mapping of incoming and outgoing data at their nodes. If all nodes or not all but a subset of the nodes in a channel are colluding, it is trivial for them to forge proof of work and cheat for system incentives.  

## Sybil Attacks

Sybil attack refers to the case where malicious node pretends to be multiple users. Malicious miners can pretend to deliver more copies and get paid. Physical forwarding is done by creating multiple Sybil identities, but only transmitting data once.

## Double Spending Attacks

Double spending attack refers to the case where the same token is spent more than once. In classic blockchain systems, nodes prevent double-spending attack by consensus to confirm the transaction sequence.

## Denial of Service Attacks

An off-line resource centric attack is known as a denial of service attack (DoS). For example, an attacker may
want to target a specific account and prevent the account holder from posting transactions.

## Quality of Service Attacks

Some attackers want to slow down the system performance, potentially reducing the amount of network connectivity and data transfer speed.

## Eclipse Attacks

Attacker controls the peer-to-peer communication network and manipulates a node's neighbors so that it only communicates with malicious nodes. The vulnerability of the network to eclipse attack depends on the peer sampling algorithm, and can be reduced by carefully choosing neighbors.

## Selfish Mining Attacks

In a selfish mining strategy, the selfish miners maintain two blockchains, one public and one private. Initially, the private blockchain is the same as the public blockchain. The attacker always mine on the private chain, unless the length of the private chain is being caught up by the public chain, in which case the attacker publishes the private chain to get rewards. The attack essentially lower the threshold of 51% attack as it may be more effiient for other miners to mine the private chain than the public chain. Yet, as an economic attack, selfish mining attack needs to be announced in advance to attract miners.

## Fraud Attacks

Malicious miners can claim large amounts of data to be transmitted but efficiently generate data on-demand using applets. If the applet is smaller than the actual amount of relay data, it increases the likelihood of malicious miners getting block bonus.