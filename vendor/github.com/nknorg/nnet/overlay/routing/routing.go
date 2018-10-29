package routing

import (
	"errors"

	"github.com/nknorg/nnet/common"
	"github.com/nknorg/nnet/log"
	"github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/util"
)

// Router is an abstract routing layer that determines how to route a message
type Router interface {
	Start() error
	Stop(error)
	ApplyMiddleware(interface{}) error
	GetNodeToRoute(remoteMsg *node.RemoteMessage) (localNode *node.LocalNode, remoteNodes []*node.RemoteNode, err error)
	SendMessage(router Router, remoteMsg *node.RemoteMessage, hasReply bool) (replyChan <-chan *node.RemoteMessage, success bool, err error)
}

// Routing is the base struct for all routing
type Routing struct {
	localMsgChan chan<- *node.RemoteMessage
	rxMsgChan    <-chan *node.RemoteMessage
	*middlewareStore
	common.LifeCycle
}

// NewRouting creates a new routing
func NewRouting(localMsgChan chan<- *node.RemoteMessage, rxMsgChan <-chan *node.RemoteMessage) (*Routing, error) {
	r := &Routing{
		localMsgChan:    localMsgChan,
		rxMsgChan:       rxMsgChan,
		middlewareStore: newMiddlewareStore(),
	}
	return r, nil
}

// Start starts the message handling process
func (r *Routing) Start(router Router, numWorkers int) error {
	r.StartOnce.Do(func() {
		for i := 0; i < numWorkers; i++ {
			go r.handleMsg(router)
		}
	})

	return nil
}

// Stop stops the routing
func (r *Routing) Stop(err error) {
	r.StopOnce.Do(func() {
		if err != nil {
			log.Warningf("Routing stops because of error: %s", err)
		} else {
			log.Infof("Routing stops")
		}

		r.LifeCycle.Stop()
	})
}

// SendMessage sends msg to the best next hop, returns reply chan (nil if if
// hasReply is false), if send success (which is true if successfully send
// message to at least one next hop), and aggregated errors during message
// sending
func (r *Routing) SendMessage(router Router, remoteMsg *node.RemoteMessage, hasReply bool) (<-chan *node.RemoteMessage, bool, error) {
	var shouldCallNextMiddleware bool
	success := false

	localNode, remoteNodes, err := router.GetNodeToRoute(remoteMsg)
	if err != nil {
		return nil, false, err
	}

	for _, f := range r.middlewareStore.remoteMessageRouted {
		remoteMsg, localNode, remoteNodes, shouldCallNextMiddleware = f(remoteMsg, localNode, remoteNodes)
		if remoteMsg == nil || !shouldCallNextMiddleware {
			break
		}
	}

	if remoteMsg == nil {
		return nil, false, nil
	}

	if localNode == nil && len(remoteNodes) == 0 {
		return nil, false, errors.New("No node to route")
	}

	if localNode != nil {
		err = r.sendMessageToLocalNode(remoteMsg, localNode)
		if err != nil {
			return nil, false, err
		}
		success = true
	}

	var replyChan <-chan *node.RemoteMessage
	errs := util.NewErrors()

	for _, remoteNode := range remoteNodes {
		// If there are multiple next hop, we only grab the first reply channel
		// because all msg have the same ID and will be using the same reply channel
		if hasReply && replyChan == nil {
			replyChan, err = remoteNode.SendMessage(remoteMsg.Msg, true)
		} else {
			_, err = remoteNode.SendMessage(remoteMsg.Msg, false)
		}

		if err != nil {
			errs = append(errs, err)
		} else {
			success = true
		}
	}

	if !success {
		return nil, false, errs.Merged()
	}

	return replyChan, success, nil
}

// sendMessageToLocalNode handles msg sent to local node
func (r *Routing) sendMessageToLocalNode(remoteMsg *node.RemoteMessage, localNode *node.LocalNode) error {
	added, err := localNode.AddToRxCache(remoteMsg)
	if !added || err != nil {
		return err
	}

	var shouldCallNextMiddleware bool

	for _, f := range r.middlewareStore.remoteMessageReceived {
		remoteMsg, shouldCallNextMiddleware = f(remoteMsg)
		if remoteMsg == nil || !shouldCallNextMiddleware {
			break
		}
	}

	if remoteMsg == nil {
		return nil
	}

	if len(remoteMsg.Msg.ReplyToId) > 0 {
		replyChan, ok := localNode.GetReplyChan(remoteMsg.Msg.ReplyToId)
		if ok && replyChan != nil {
			select {
			case replyChan <- remoteMsg:
			default:
				log.Warning("Reply chan unavailable or full, discarding msg")
			}
		}
		return nil
	}

	select {
	case r.localMsgChan <- remoteMsg:
	default:
		log.Warning("Router local msg chan full, discarding msg")
	}

	return nil
}

// handleMsg starts to read received message from rxMsgChan, compute the route
// and dispatch message to local or remote nodes
func (r *Routing) handleMsg(router Router) {
	var remoteMsg *node.RemoteMessage
	var shouldCallNextMiddleware bool
	var err error

	for {
		if r.IsStopped() {
			return
		}

		remoteMsg = <-r.rxMsgChan

		for _, f := range r.middlewareStore.remoteMessageArrived {
			remoteMsg, shouldCallNextMiddleware = f(remoteMsg)
			if remoteMsg == nil || !shouldCallNextMiddleware {
				break
			}
		}

		if remoteMsg == nil {
			continue
		}

		_, _, err = r.SendMessage(router, remoteMsg, false)
		if err != nil {
			log.Warning(err)
		}
	}
}
