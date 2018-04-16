# Moore type cellular automata gossip and ising consensus condition simulation introduction

This simulation demonstrates the spread of a gossip and Ising consensus with the cellular automaton model. It consists of a number of identical cells representing peers in the network arranged in a regular grid. Each cell can be in one of two states - for example "on" or "off", or "heard" or "not heard". The states indicate settings or actions of cells. Timeindex goes through the simulation in steps. At each time step, the state of each cell may change. The state of a cell after any time step is determined by a series of rules that indicate how that state depends on the previous state of that cell and the states of the immediate neighbors of the cell. The same rules are used to update the state of each cell in the grid. The cellular automata model is therefore homogeneous in terms of rules.

## Rules for cellular automata gossip and ising consensus

Cellular automata model local interactions well. Therefore, the simulation is divided into two phases:

- Cellular automata gossip to a predefined percentage. A gossip (transaction) spreads by peers only talk to their immediate neighbors. This kind interaction is local and could be modeled with a cellular automata. Here peers are modeled as cells/patches in netlogo.Each cell has two states: ignorance about the item of gossip or knowing the gossip. And we define colour black indicating a cell that does not know the gossip and color white one that does. Local interaction between peer and neighbors is modelled using the rules of cellular automata as: If the cell is black, and it has one or more white neighbours, consider each white neighbour in turn. For each white neighbour, change to white with some specific probability, otherwise remain black. If the cell is white already, the cell remains white. A gloal gossip percentage variable is defined so that the gossip process can stop at a give percentage to proceed with next step: Ising model consensus.Cellular automata gossip stops at a predefined percentage. A gossip (transaction) spreads from peers only with their immediate neighbors. This type of interaction is local and could be modeled using a cellular automaton. Here, peers are modeled as cells / patches in Netlogo. Each cell has two states: ignorance of gossip or knowing gossip. And we define color black, which indicates a cell that does not know the gossip and color white knows what that does. The local interaction between peer and neighbor is modeled using the rules of cellular automata: if the cell is black and has one or more white neighbors, look at each white neighbor in turn. For every white neighbor, the cell likely to switch to white, or the cell will stay black. If the cell is already white, the cell stays white. A global gossip percentage variable is defined so that the gossip process can stop at a given percentage to proceed to the next step: Ising model consensus.

-To facilitate the process combination of gossip and ising consensus, we define color "white" is vote=1; and color "black" as vote =0 as below

```
to change-state-vote  ;cell state changing procedure
  ifelse vote = 1
    [ set pcolor white  ]
    [ set pcolor black  ]
end
```

- Celluar automata based ising model consensus was first proposed by NKN in whitepaper with solid mathematical proofs. Here, in ising model consensus simulation, we define a cell changes state (black or white) according to the joint states of all of its neighbours.Peers might adopt a vote only if the majority of their neighbors have already adopted it. Also, we define that a cell has a Moore type neighborhood, which indicates that each cell has eight neighbors around. Base on these conditions, the rule of cellular automata can be set as: the new state of a cell is the state of the majority of the cell¡¯s Moore neighbours, or the cell¡¯s previous state if the neighbours are equally divided between black and white, indicating ignorance of gossip or knowing gossip. Since there are eight Moore neighbors. To reach ising consensus, we define a threshold of sum of neighbor vote count as K/2, where K denotes the total number of neighbors. Thus, the rule says that a cell is black if there are four or more black cells surrounding it, white if there are four or more white cells around it, or remains in its previous state if there are four black and four white neighbors.BTW, the initial state of this consensus is defined by the phase 1 where a partial gossip process at a given gossip percentage.


```
   ask patches
    [ let last-vote vote
      if ncount > 4 [
     set vote 1
     ]; Moore type neighbor count K=8, K/2=4; if more than K/2=4 vote 1 then center cell vote 1
      if ncount < 4 [
      set vote 0

    ]; if less than K/2=4 vote 0 then center cell vote 0
      if ncount = 4;
        [
        ;set vote (1 - vote);if K/2 do not change, then center cell do not change.
        set vote 1; if K/2 do not change, then center cell change to set vote 1.
    ]
       if vote != last-vote; if center cell state does not change
        [ set any-votes-changed? true ]; change stabalization indicator
      change-state-vote ]
```

- With "fault-tor", a initial fault-torlerance can be set up by a slider before ising consensus

```
 ask patches
    [ if first-heard >= 0[
      ifelse random 100 < fault-tor
      [ set vote 0 ]
      [ set vote 1 ]
      change-state-vote
      ]
  ]
```

## Simulation Results Analysis

- For the process of cellular automata gossip, it is interesting to observe the effect of the different probabilities of communication of gossip by making a corresponding modification of the rules. The simulation shows that the spread of gossip by local, peer-to-peer Interactions are not seriously hampered by a low probability of transmission at any given time, although low probabilities result in slow propagation.

- For ising model consensus, setting a threshold value of K/2 can gurantee the consensus can be reached globally. However, it is esstential to define proper voting rule and randomize gossip distribution to enhance fault tolerance and covergence time.


Cellular automata with 4 neighbors vote 1 then it votes 1 when fault toerlance is 30% before ising consensus. It results in consensus is achieved.
![Cellular automata with 4 neighbors vote 1 then it votes 1 when fault toerlance is 30%](https://github.com/barbapapait/misc/blob/master/30percent_ising_consensus.PNG)

Cellular automata with 4 neighbors vote 1 then it votes 1 when fault toerlance is 65% before ising consensus. It results in consensus is achieved but with slow convergence time.
![Cellular automata with 4 neighbors vote 1 then it votes 1 when fault toerlance is 65%](https://github.com/barbapapait/misc/blob/master/65percent_ising_consensus.PNG)

Cellular automata with 4 neighbors vote 1 then it votes 1 when fault toerlance is 68% before ising consensus. It fails to achieve consensus for most of cases, indicating a near to a threshold.
![Cellular automata with 4 neighbors vote 1 then it votes 1 when fault toerlance is 68% in initial state](https://github.com/barbapapait/misc/blob/master/68percent_ising_consensus.PNG)

Cellular automata with 4 neighbors vote 1 then it votes 1 when fault toerlance is 68% before ising consensus. It indicates a final state that consensus cannot be achieved.
![Cellular automata with 4 neighbors vote 1 then it votes 1 when fault toerlance is 68% in final state](https://github.com/barbapapait/misc/blob/master/68percent_ising_consensus_failed.PNG)

## Credits and References

* Wilensky, U. (1999). NetLogo. http://ccl.northwestern.edu/netlogo/. Center for Connected Learning and Computer-Based Modeling, Northwestern University, Evanston, IL.

* Wilensky, U. (1997).  NetLogo Rumor Mill model.  http://ccl.northwestern.edu/netlogo/models/RumorMill.  Center for Connected Learning and Computer-Based Modeling, Northwestern University, Evanston, IL.

* Wilensky, U. (1998).  NetLogo Voting model.  http://ccl.northwestern.edu/netlogo/models/Voting.  Center for Connected Learning and Computer-Based Modeling, Northwestern University, Evanston, IL.
