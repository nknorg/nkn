# Simulation of cellular automata consensus algorithm

To show the performance of our consensus algorithm, we apply it to a
simulated network with N=1,000,000 nodes. Each node has k neighbors
randomly selected from the network. At each iteration, its state is
updated based on the states of its k neighbors plus its own state
using MVCA rule as proposed above. Neighbors are one directional so
that J is not guaranteed to be symmetric. Initially (iteration 0), the
state of each node is independently chosen to be 1 or -1 with some
probability. The simulation is run for several iterations with
different k, as shown in Fig.1. One can see that the MVCA converges to
global consensus state in just a few steps even with k = 10, much
faster than the theoretical upper bound kN. Note that consensus will
be reached even when the initial state contains equal number of 1 and
-1 nodes. Larger k leads to faster convergence.


Fig.1, average state of the system converges to either 1 or -1, both
representing global consensus. MVCA converges to consensus state which
is the state of the majority nodes in just a few steps even with only
10 neighboring nodes in a 1,000,000 nodes network. Increasing the
number of neighbors accelerates convergence. Note that when exactly
half of the nodes are in one state while the other half in the other
state, the converged state could be either one.  ![ising consensus
simulation convergence](convergence.svg)

It should be mentioned that when N is large, the topology of the
random network will be closer to its "typical" case as the probability
to have any specific connectivity decreases exponentially as N
increases. Thus, one should look at mean convergence time rather than
worst case when (and only when) N is large, as in our simulations.

We further simulate the scenario where a fraction of nodes are
malicious. In this case "correct" nodes have initial state 1, while
malicious nodes have initial state -1 and does not update their states
regardless of the states of their neighbors. The goal of the
¡°correct¡± nodes is to reach consensus on state 1, while the
malicious nodes try to reach consensus on state -1. From the results
in Fig.2, it can be seen that there is a transition between collapsing
to the wrong state (-1) and keeping most correct nodes at the correct
state (1). For N = 1,000,000 and k = 10, the critical fraction of the
malicious nodes is around 30%, which is significant considering the
size of N. The critical fraction also depends on k, as shown in
Fig.2. Larger k has two effects: more malicious nodes can be
tolerated, and less correct nodes will be affected by malicious nodes.

Fig.2, fraction of correct nodes (state 1) under the attack of
malicious nodes (state -1) that does not update their states. There is
a transition between whether the system will collapse to the wrong
state when the initial fraction of malicious nodes changes.  ![ising
consensus simulation malicious](malicious.svg)

Results in Fig.1 and Fig.2 combined show the upper bound and lower
bound of the network dynamics with faulty initial states: the former
one simulates the case where nodes with incorrect initial state are
not malicious, while the latter one simulates the case where nodes
with incorrect initial state are all malicious and want the rest of
the network to agree on the incorrect state. Network dynamics fall
between these two curves with the same initial states whatever
strategy faulty nodes (the ones with faulty initial state) take.

One advantage of using Ising model to describe the system is the
natural extension to noisy and unreliable communication channels. The
temperature parameter in Ising model controls the amount of noise in
the system, and in our case is the randomness in the updating rule. At
zero temperature, the updating rule is deterministic, while the rule
becomes more random when temperature increases, and eventually becomes
purely random when temperature goes to infinity. By including a
default state, probabilistic failure of message delivery can be
modeled by finite temperature in Ising model. Thus, consensus can
still be made as long as noise is under the threshold, as discussed
above. The threshold can be computed numerically given the statistics
of network connectivity. Asynchronous state update can also be modeled
by such noise when communication timeout is added, making it practical
for implementation.
