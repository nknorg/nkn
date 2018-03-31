globals [           ;define global variables
  heard-count            ;how many cells have heard the gossip
  vote-count
  gossip-percent    ;percentage of gossip spread

]

patches-own [     ; define cellular automata variables
  times-heard     ; tracks times the gossip has been heard
  first-heard     ;clock tick when first heard the gossip
  just-heard?     ;tracks whether gossip was heard this round -- resets each round
  vote   ; center cell vote (0 or 1)
  ncount  ; sum of votes of neighbors around the center cell, in Moore type neighborhood, there are 8 neighbors
]

; single transaction (TX) creation setup procedures

to setup [transaction?]
  clear-all
  set heard-count 0
  ask patches
    [ set first-heard -1
      set times-heard 0
      set just-heard? false
      change-state ]
     transaction
    reset-ticks
end

to transaction
  ; tell the center cell the gossip
  ask patch 0 0
    [ hear-gossip 0 ]
end

to go
  if all? patches [gossip-percent > 0.75]; if percentage of gossip reach 75% then gossip stops
   [ stop ]; stop gossip
  ask patches
    [ if times-heard > 0
        [ spread-gossip ] ]
  update
  tick
end

to spread-gossip  ;; cell procedure
  let neighbor nobody
   set neighbor one-of neighbors; set up Moore type nerighbors: eight neighors
  ask neighbor [ set just-heard? true ]
end

to hear-gossip [when]  ;; cell procedure
  if first-heard = -1
    [ set first-heard when
      set just-heard? true ]
  set times-heard times-heard + 1
  change-state
end

to update
  ask patches with [just-heard?]
    [ set just-heard? false
      hear-gossip ticks ]
  set heard-count count patches with [times-heard > 0]
  set vote-count heard-count
  set gossip-percent heard-count / count patches
end

; changing state procedures

to change-state  ;cell state changing procedure
  ifelse first-heard >= 0
    [ set pcolor white ]
    [ set pcolor black ]
end

to randomize

  ask patches
    [ if first-heard >= 0[
      set vote random 2
      change-state-vote
      ]
  ]
  reset-ticks
end

to consensus
     ; keep track of whether any cell has changed their vote
  let any-votes-changed? false
    ask patches
    [ set ncount (sum [vote] of neighbors) ]
   ; before any cells change their votes
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

    set heard-count count patches

  ;; if the votes have stabilized, stop the simulation
   if not any-votes-changed? [ stop ]
   tick
   update-vote
end

to change-state-vote  ;cell state changing procedure
  ifelse vote = 1
    [ set pcolor white  ]
    [ set pcolor black  ]
end

to update-vote
  set vote-count count patches with [vote = 1]
  set gossip-percent vote-count / count patches
end
@#$#@#$#@
GRAPHICS-WINDOW
55
10
550
506
-1
-1
2.423
1
12
1
1
1
0
1
1
1
-100
100
-100
100
1
1
1
ticks
1000.0

PLOT
550
10
1089
506
Cellular Automata Gossip and Consensus Spread
Time Step
Gossip Percent
0.0
20.0
0.0
100.0
true
false
"" ""
PENS
"default" 1.0 0 -16777216 true "" "plot (vote-count / count patches) * 100"

BUTTON
575
28
686
61
1. TX Creation
setup true
NIL
1
T
OBSERVER
NIL
NIL
NIL
NIL
1

BUTTON
575
65
700
99
2. Gossip Spread
go
T
1
T
OBSERVER
NIL
NIL
NIL
NIL
0

BUTTON
575
140
713
173
4. Ising-Consensus
consensus
T
1
T
OBSERVER
NIL
NIL
NIL
NIL
0

BUTTON
575
101
681
136
3. Randomize
randomize
NIL
1
T
OBSERVER
NIL
NIL
NIL
NIL
0

@#$#@#$#@
## Simulation Introduction

This simulation demonstrates the spread of a gossip and Ising consensus with the cellular automaton model. It consists of a number of identical cells representing peers in the network arranged in a regular grid. Each cell can be in one of two states - for example "on" or "off", or "heard" or "not heard". The states indicate settings or actions of cells. Timeindex goes through the simulation in steps. At each time step, the state of each cell may change. The state of a cell after any time step is determined by a series of rules that indicate how that state depends on the previous state of that cell and the states of the immediate neighbors of the cell. The same rules are used to update the state of each cell in the grid. The cellular automata model is therefore homogeneous in terms of rules.

Cellular automata model local interactions well. Therefore, the simulation is divided into two phases:

1. Cellular automata gossip to a predefined percentage. A gossip (transaction) spreads by peers only talk to their immediate neighbors. This kind interaction is local and could be modeled with a cellular automata. Here peers are modeled as cells/patches in netlogo.Each cell has two states: ignorance about the item of gossip or knowing the gossip. And we define colour black indicating a cell that does not know the gossip and color white one that does. Local interaction between peer and neighbors is modelled using the rules of cellular automata as: If the cell is black, and it has one or more white neighbours, consider each white neighbour in turn. For each white neighbour, change to white with some specific probability, otherwise remain black. If the cell is white already, the cell remains white. A gloal gossip percentage variable is defined so that the gossip process can stop at a give percentage to proceed with next step: Ising model consensus.Cellular automata gossip stops at a predefined percentage. A gossip (transaction) spreads from peers only with their immediate neighbors. This type of interaction is local and could be modeled using a cellular automaton. Here, peers are modeled as cells / patches in Netlogo. Each cell has two states: ignorance of gossip or knowing gossip. And we define color black, which indicates a cell that does not know the gossip and color white knows what that does. The local interaction between peer and neighbor is modeled using the rules of cellular automata: if the cell is black and has one or more white neighbors, look at each white neighbor in turn. For every white neighbor, the cell likely to switch to white, or the cell will stay black. If the cell is already white, the cell stays white. A global gossip percentage variable is defined so that the gossip process can stop at a given percentage to proceed to the next step: Ising model consensus.

2. Celluar automata based ising model consensus was first proposed by NKN in whitepaper with solid mathematical proofs. Here, in ising model consensus simulation, we define a cell changes state (black or white) according to the joint states of all of its neighbours.Peers might adopt a vote only if the majority of their neighbors have already adopted it. Also, we define that a cell has a Moore type neighborhood, which indicates that each cell has eight neighbors around. Base on these conditions, the rule of cellular automata can be set as: the new state of a cell is the state of the majority of the cell’s Moore neighbours, or the cell’s previous state if the neighbours are equally divided between black and white, indicating ignorance of gossip or knowing gossip. Since there are eight Moore neighbors. To reach ising consensus, we define a threshold of sum of neighbor vote count as K/2, where K denotes the total number of neighbors. Thus, the rule says that a cell is black if there are four or more black cells surrounding it, white if there are four or more white cells around
it, or remains in its previous state if there are four black and four white neighbors.BTW, the initial state of this consensus is defined by the phase 1 where a partial gossip process at a given gossip percentage.


## Simulation Results Analysis

1. For the process of cellular automata gossip, it is interesting to observe the effect of the different probabilities of communication of gossip by making a corresponding modification of the rules. The simulation shows that the spread of gossip by local, peer-to-peer Interactions are not seriously hampered by a low probability of transmission at any given time, although low probabilities result in slow propagation.

2. For ising model consensus, setting a threshold value of K/2 can gurantee the consensus can be reached globally. However, it is esstential to define proper voting rule and randomize gossip distribution to enhance fault tolerance and covergence time.
@#$#@#$#@
default
true
0
Polygon -7500403 true true 150 5 40 250 150 205 260 250

airplane
true
0
Polygon -7500403 true true 150 0 135 15 120 60 120 105 15 165 15 195 120 180 135 240 105 270 120 285 150 270 180 285 210 270 165 240 180 180 285 195 285 165 180 105 180 60 165 15

arrow
true
0
Polygon -7500403 true true 150 0 0 150 105 150 105 293 195 293 195 150 300 150

box
false
0
Polygon -7500403 true true 150 285 285 225 285 75 150 135
Polygon -7500403 true true 150 135 15 75 150 15 285 75
Polygon -7500403 true true 15 75 15 225 150 285 150 135
Line -16777216 false 150 285 150 135
Line -16777216 false 150 135 15 75
Line -16777216 false 150 135 285 75

bug
true
0
Circle -7500403 true true 96 182 108
Circle -7500403 true true 110 127 80
Circle -7500403 true true 110 75 80
Line -7500403 true 150 100 80 30
Line -7500403 true 150 100 220 30

butterfly
true
0
Polygon -7500403 true true 150 165 209 199 225 225 225 255 195 270 165 255 150 240
Polygon -7500403 true true 150 165 89 198 75 225 75 255 105 270 135 255 150 240
Polygon -7500403 true true 139 148 100 105 55 90 25 90 10 105 10 135 25 180 40 195 85 194 139 163
Polygon -7500403 true true 162 150 200 105 245 90 275 90 290 105 290 135 275 180 260 195 215 195 162 165
Polygon -16777216 true false 150 255 135 225 120 150 135 120 150 105 165 120 180 150 165 225
Circle -16777216 true false 135 90 30
Line -16777216 false 150 105 195 60
Line -16777216 false 150 105 105 60

car
false
0
Polygon -7500403 true true 300 180 279 164 261 144 240 135 226 132 213 106 203 84 185 63 159 50 135 50 75 60 0 150 0 165 0 225 300 225 300 180
Circle -16777216 true false 180 180 90
Circle -16777216 true false 30 180 90
Polygon -16777216 true false 162 80 132 78 134 135 209 135 194 105 189 96 180 89
Circle -7500403 true true 47 195 58
Circle -7500403 true true 195 195 58

circle
false
0
Circle -7500403 true true 0 0 300

circle 2
false
0
Circle -7500403 true true 0 0 300
Circle -16777216 true false 30 30 240

cow
false
0
Polygon -7500403 true true 200 193 197 249 179 249 177 196 166 187 140 189 93 191 78 179 72 211 49 209 48 181 37 149 25 120 25 89 45 72 103 84 179 75 198 76 252 64 272 81 293 103 285 121 255 121 242 118 224 167
Polygon -7500403 true true 73 210 86 251 62 249 48 208
Polygon -7500403 true true 25 114 16 195 9 204 23 213 25 200 39 123

cylinder
false
0
Circle -7500403 true true 0 0 300

dot
false
0
Circle -7500403 true true 90 90 120

face happy
false
0
Circle -7500403 true true 8 8 285
Circle -16777216 true false 60 75 60
Circle -16777216 true false 180 75 60
Polygon -16777216 true false 150 255 90 239 62 213 47 191 67 179 90 203 109 218 150 225 192 218 210 203 227 181 251 194 236 217 212 240

face neutral
false
0
Circle -7500403 true true 8 7 285
Circle -16777216 true false 60 75 60
Circle -16777216 true false 180 75 60
Rectangle -16777216 true false 60 195 240 225

face sad
false
0
Circle -7500403 true true 8 8 285
Circle -16777216 true false 60 75 60
Circle -16777216 true false 180 75 60
Polygon -16777216 true false 150 168 90 184 62 210 47 232 67 244 90 220 109 205 150 198 192 205 210 220 227 242 251 229 236 206 212 183

fish
false
0
Polygon -1 true false 44 131 21 87 15 86 0 120 15 150 0 180 13 214 20 212 45 166
Polygon -1 true false 135 195 119 235 95 218 76 210 46 204 60 165
Polygon -1 true false 75 45 83 77 71 103 86 114 166 78 135 60
Polygon -7500403 true true 30 136 151 77 226 81 280 119 292 146 292 160 287 170 270 195 195 210 151 212 30 166
Circle -16777216 true false 215 106 30

flag
false
0
Rectangle -7500403 true true 60 15 75 300
Polygon -7500403 true true 90 150 270 90 90 30
Line -7500403 true 75 135 90 135
Line -7500403 true 75 45 90 45

flower
false
0
Polygon -10899396 true false 135 120 165 165 180 210 180 240 150 300 165 300 195 240 195 195 165 135
Circle -7500403 true true 85 132 38
Circle -7500403 true true 130 147 38
Circle -7500403 true true 192 85 38
Circle -7500403 true true 85 40 38
Circle -7500403 true true 177 40 38
Circle -7500403 true true 177 132 38
Circle -7500403 true true 70 85 38
Circle -7500403 true true 130 25 38
Circle -7500403 true true 96 51 108
Circle -16777216 true false 113 68 74
Polygon -10899396 true false 189 233 219 188 249 173 279 188 234 218
Polygon -10899396 true false 180 255 150 210 105 210 75 240 135 240

house
false
0
Rectangle -7500403 true true 45 120 255 285
Rectangle -16777216 true false 120 210 180 285
Polygon -7500403 true true 15 120 150 15 285 120
Line -16777216 false 30 120 270 120

leaf
false
0
Polygon -7500403 true true 150 210 135 195 120 210 60 210 30 195 60 180 60 165 15 135 30 120 15 105 40 104 45 90 60 90 90 105 105 120 120 120 105 60 120 60 135 30 150 15 165 30 180 60 195 60 180 120 195 120 210 105 240 90 255 90 263 104 285 105 270 120 285 135 240 165 240 180 270 195 240 210 180 210 165 195
Polygon -7500403 true true 135 195 135 240 120 255 105 255 105 285 135 285 165 240 165 195

line
true
0
Line -7500403 true 150 0 150 300

line half
true
0
Line -7500403 true 150 0 150 150

pentagon
false
0
Polygon -7500403 true true 150 15 15 120 60 285 240 285 285 120

person
false
0
Circle -7500403 true true 110 5 80
Polygon -7500403 true true 105 90 120 195 90 285 105 300 135 300 150 225 165 300 195 300 210 285 180 195 195 90
Rectangle -7500403 true true 127 79 172 94
Polygon -7500403 true true 195 90 240 150 225 180 165 105
Polygon -7500403 true true 105 90 60 150 75 180 135 105

plant
false
0
Rectangle -7500403 true true 135 90 165 300
Polygon -7500403 true true 135 255 90 210 45 195 75 255 135 285
Polygon -7500403 true true 165 255 210 210 255 195 225 255 165 285
Polygon -7500403 true true 135 180 90 135 45 120 75 180 135 210
Polygon -7500403 true true 165 180 165 210 225 180 255 120 210 135
Polygon -7500403 true true 135 105 90 60 45 45 75 105 135 135
Polygon -7500403 true true 165 105 165 135 225 105 255 45 210 60
Polygon -7500403 true true 135 90 120 45 150 15 180 45 165 90

square
false
0
Rectangle -7500403 true true 30 30 270 270

square 2
false
0
Rectangle -7500403 true true 30 30 270 270
Rectangle -16777216 true false 60 60 240 240

star
false
0
Polygon -7500403 true true 151 1 185 108 298 108 207 175 242 282 151 216 59 282 94 175 3 108 116 108

target
false
0
Circle -7500403 true true 0 0 300
Circle -16777216 true false 30 30 240
Circle -7500403 true true 60 60 180
Circle -16777216 true false 90 90 120
Circle -7500403 true true 120 120 60

tree
false
0
Circle -7500403 true true 118 3 94
Rectangle -6459832 true false 120 195 180 300
Circle -7500403 true true 65 21 108
Circle -7500403 true true 116 41 127
Circle -7500403 true true 45 90 120
Circle -7500403 true true 104 74 152

triangle
false
0
Polygon -7500403 true true 150 30 15 255 285 255

triangle 2
false
0
Polygon -7500403 true true 150 30 15 255 285 255
Polygon -16777216 true false 151 99 225 223 75 224

truck
false
0
Rectangle -7500403 true true 4 45 195 187
Polygon -7500403 true true 296 193 296 150 259 134 244 104 208 104 207 194
Rectangle -1 true false 195 60 195 105
Polygon -16777216 true false 238 112 252 141 219 141 218 112
Circle -16777216 true false 234 174 42
Rectangle -7500403 true true 181 185 214 194
Circle -16777216 true false 144 174 42
Circle -16777216 true false 24 174 42
Circle -7500403 false true 24 174 42
Circle -7500403 false true 144 174 42
Circle -7500403 false true 234 174 42

turtle
true
0
Polygon -10899396 true false 215 204 240 233 246 254 228 266 215 252 193 210
Polygon -10899396 true false 195 90 225 75 245 75 260 89 269 108 261 124 240 105 225 105 210 105
Polygon -10899396 true false 105 90 75 75 55 75 40 89 31 108 39 124 60 105 75 105 90 105
Polygon -10899396 true false 132 85 134 64 107 51 108 17 150 2 192 18 192 52 169 65 172 87
Polygon -10899396 true false 85 204 60 233 54 254 72 266 85 252 107 210
Polygon -7500403 true true 119 75 179 75 209 101 224 135 220 225 175 261 128 261 81 224 74 135 88 99

wheel
false
0
Circle -7500403 true true 3 3 294
Circle -16777216 true false 30 30 240
Line -7500403 true 150 285 150 15
Line -7500403 true 15 150 285 150
Circle -7500403 true true 120 120 60
Line -7500403 true 216 40 79 269
Line -7500403 true 40 84 269 221
Line -7500403 true 40 216 269 79
Line -7500403 true 84 40 221 269

x
false
0
Polygon -7500403 true true 270 75 225 30 30 225 75 270
Polygon -7500403 true true 30 75 75 30 270 225 225 270
@#$#@#$#@
NetLogo 6.0.3
@#$#@#$#@
setup true
repeat 90 [ go ]
@#$#@#$#@
@#$#@#$#@
@#$#@#$#@
@#$#@#$#@
default
0.0
-0.2 0 0.0 1.0
0.0 1 1.0 0.0
0.2 0 0.0 1.0
link direction
true
0
Line -7500403 true 150 150 90 180
Line -7500403 true 150 150 210 180
@#$#@#$#@
0
@#$#@#$#@
