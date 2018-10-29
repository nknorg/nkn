package chord

import (
	"errors"

	"github.com/nknorg/nnet/log"
	"github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/util"
)

// Connect connects to a remote node. optionally with id info to check if
// connection has established
func (c *Chord) Connect(addr string, id []byte) error {
	if id != nil {
		remoteNode := c.neighbors.GetByID(id)
		if remoteNode != nil {
			log.Infof("Node with id %x is already a neighbor", id)
			return c.addRemoteNode(remoteNode)
		}
	}

	remoteNode, ready, err := c.LocalNode.Connect(addr)
	if err != nil {
		return err
	}

	if ready {
		return c.addRemoteNode(remoteNode)
	}

	return nil
}

// addSuccessor adds a remote node to the successor list of chord overlay
func (c *Chord) addSuccessor(remoteNode *node.RemoteNode) error {
	if !c.successors.Exists(remoteNode.Id) {
		added, replaced, err := c.successors.AddOrReplace(remoteNode)
		if err != nil {
			return err
		}

		if added {
			index := c.successors.GetIndex(remoteNode.Id)
			if index >= 0 {
				for _, f := range c.middlewareStore.successorAdded {
					if !f(remoteNode, index) {
						break
					}
				}
			}
		}

		if replaced != nil {
			for _, f := range c.middlewareStore.successorRemoved {
				if !f(replaced) {
					break
				}
			}

			c.maybeStopRemoteNode(replaced)
		}
	}

	return nil
}

// addPredecessor adds a remote node to the predecessor list of chord overlay
func (c *Chord) addPredecessor(remoteNode *node.RemoteNode) error {
	if !c.predecessors.Exists(remoteNode.Id) {
		added, replaced, err := c.predecessors.AddOrReplace(remoteNode)
		if err != nil {
			return err
		}

		if added {
			index := c.predecessors.GetIndex(remoteNode.Id)
			if index >= 0 {
				for _, f := range c.middlewareStore.predecessorAdded {
					if !f(remoteNode, index) {
						break
					}
				}
			}
		}

		if replaced != nil {
			for _, f := range c.middlewareStore.predecessorRemoved {
				if !f(replaced) {
					break
				}
			}

			c.maybeStopRemoteNode(replaced)
		}
	}

	return nil
}

// addFingerTable adds a remote node to the finger table list of chord overlay
func (c *Chord) addFingerTable(remoteNode *node.RemoteNode, index int) error {
	finger := c.fingerTable[index]

	if !finger.Exists(remoteNode.Id) {
		added, replaced, err := finger.AddOrReplace(remoteNode)
		if err != nil {
			return err
		}

		if added {
			i := finger.GetIndex(remoteNode.Id)
			if i >= 0 {
				for _, f := range c.middlewareStore.fingerTableAdded {
					if !f(remoteNode, index, i) {
						break
					}
				}
			}
		}

		if replaced != nil {
			for _, f := range c.middlewareStore.fingerTableRemoved {
				if !f(replaced, index) {
					break
				}
			}

			c.maybeStopRemoteNode(replaced)
		}

		if added != (replaced != nil) {
			c.updateSuccPredMaxNumNodes()
		}
	}

	return nil
}

// addNeighbor adds a remote node to the neighbor list of chord overlay
func (c *Chord) addNeighbor(remoteNode *node.RemoteNode) error {
	if !c.neighbors.Exists(remoteNode.Id) {
		added, replaced, err := c.neighbors.AddOrReplace(remoteNode)
		if err != nil {
			return err
		}

		if added {
			index := c.neighbors.GetIndex(remoteNode.Id)
			if index >= 0 {
				for _, f := range c.middlewareStore.neighborAdded {
					if !f(remoteNode, index) {
						break
					}
				}
			}
		}

		if replaced != nil {
			for _, f := range c.middlewareStore.neighborRemoved {
				if !f(replaced) {
					break
				}
			}

			c.maybeStopRemoteNode(replaced)
		}
	}

	return nil
}

// addRemoteNode adds a remote node to the neighbor lists of chord overlay
func (c *Chord) addRemoteNode(remoteNode *node.RemoteNode) error {
	if !remoteNode.IsReady() {
		return errors.New("Remote node is not ready yet")
	}

	err := c.addSuccessor(remoteNode)
	if err != nil {
		log.Error(err)
	}

	err = c.addPredecessor(remoteNode)
	if err != nil {
		log.Error(err)
	}

	for i := range c.fingerTable {
		err = c.addFingerTable(remoteNode, i)
		if err != nil {
			log.Error(err)
		}
	}

	err = c.addNeighbor(remoteNode)
	if err != nil {
		log.Error(err)
	}

	return nil
}

// removeNeighbor removes a remote node from the neighbor lists of chord overlay
func (c *Chord) removeNeighbor(remoteNode *node.RemoteNode) error {
	removed := c.successors.Remove(remoteNode)
	if removed {
		for _, f := range c.middlewareStore.successorRemoved {
			if !f(remoteNode) {
				break
			}
		}

		for _, rn := range c.neighbors.ToRemoteNodeList(true) {
			if rn != remoteNode {
				err := c.addSuccessor(rn)
				if err != nil {
					log.Error(err)
				}
			}
		}
	}

	removed = c.predecessors.Remove(remoteNode)
	if removed {
		for _, f := range c.middlewareStore.predecessorRemoved {
			if !f(remoteNode) {
				break
			}
		}

		neighbors := c.neighbors.ToRemoteNodeList(true)
		for i := range neighbors {
			if neighbors[len(neighbors)-i-1] != remoteNode {
				err := c.addPredecessor(neighbors[len(neighbors)-i-1])
				if err != nil {
					log.Error(err)
				}
			}
		}
	}

	for i, finger := range c.fingerTable {
		removed = finger.Remove(remoteNode)
		if removed {
			for _, f := range c.middlewareStore.fingerTableRemoved {
				if !f(remoteNode, i) {
					break
				}
			}

			for _, rn := range c.neighbors.ToRemoteNodeList(true) {
				if rn != remoteNode {
					err := c.addFingerTable(rn, i)
					if err != nil {
						log.Error(err)
					}
				}
			}
		}
	}

	removed = c.neighbors.Remove(remoteNode)
	if removed {
		for _, f := range c.middlewareStore.neighborRemoved {
			if !f(remoteNode) {
				break
			}
		}
	}

	return nil
}

// maybeStopRemoteNode removes an outbound node that is no longer in successors,
// predecessor, or finger table
func (c *Chord) maybeStopRemoteNode(remoteNode *node.RemoteNode) bool {
	if !remoteNode.IsOutbound {
		return false
	}

	if c.successors.Exists(remoteNode.Id) {
		return false
	}

	if c.predecessors.Exists(remoteNode.Id) {
		return false
	}

	for _, finger := range c.fingerTable {
		if finger.Exists(remoteNode.Id) {
			return false
		}
	}

	remoteNode.Stop(nil)

	return true
}

func (c *Chord) updateNeighborList(neighborList *NeighborList) error {
	newNodes, err := neighborList.getNewNodesToConnect()
	if err != nil {
		return err
	}

	errs := util.NewErrors()
	for _, newNode := range newNodes {
		err = c.Connect(newNode.Addr, newNode.Id)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs.Merged()
}
