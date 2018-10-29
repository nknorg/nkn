package overlay

import (
	"errors"
	"fmt"
	"time"

	"github.com/nknorg/nnet/common"
	"github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay/routing"
	"github.com/nknorg/nnet/protobuf"
)

const (
	// Max number of msg to be processed by local node that can be buffered
	localMsgChanLen = 23333

	// Timeout for reply message
	replyTimeout = 5 * time.Second
)

// Overlay is an abstract overlay network
type Overlay struct {
	LocalNode    *node.LocalNode
	LocalMsgChan chan *node.RemoteMessage
	routers      map[protobuf.RoutingType]routing.Router
	common.LifeCycle
}

// NewOverlay creates a new overlay network
func NewOverlay(localNode *node.LocalNode) (*Overlay, error) {
	if localNode == nil {
		return nil, errors.New("Local node is nil")
	}

	overlay := &Overlay{
		LocalNode:    localNode,
		LocalMsgChan: make(chan *node.RemoteMessage, localMsgChanLen),
		routers:      make(map[protobuf.RoutingType]routing.Router),
	}
	return overlay, nil
}

// GetLocalNode returns the local node
func (ovl *Overlay) GetLocalNode() *node.LocalNode {
	return ovl.LocalNode
}

// AddRouter adds a router for a routingType, and returns error if router has
// already benn added for the type
func (ovl *Overlay) AddRouter(routingType protobuf.RoutingType, router routing.Router) error {
	_, ok := ovl.routers[routingType]
	if ok {
		return fmt.Errorf("Router for type %v is already added", routingType)
	}
	ovl.routers[routingType] = router
	return nil
}

// GetRouter gets a router for a routingType, and returns error if router of the
// type has not benn added yet
func (ovl *Overlay) GetRouter(routingType protobuf.RoutingType) (routing.Router, error) {
	router, ok := ovl.routers[routingType]
	if !ok {
		return nil, fmt.Errorf("Router for type %v has not been added yet", routingType)
	}
	return router, nil
}

// GetRouters gets a list of routers added
func (ovl *Overlay) GetRouters() []routing.Router {
	routers := make([]routing.Router, 0)
	for _, router := range ovl.routers {
		routers = append(routers, router)
	}
	return routers
}

// SetRouter sets a router for a routingType regardless of whether a router has
// been set up for this type or not
func (ovl *Overlay) SetRouter(routingType protobuf.RoutingType, router routing.Router) {
	ovl.routers[routingType] = router
}

// StartRouters starts all routers added to overlay network
func (ovl *Overlay) StartRouters() error {
	for _, router := range ovl.routers {
		err := router.Start()
		if err != nil {
			return nil
		}
	}

	return nil
}

// StopRouters stops all routers added to overlay network
func (ovl *Overlay) StopRouters(err error) {
	for _, router := range ovl.routers {
		router.Stop(err)
	}
}

// SendMessage sends msg to the best next hop, returns reply chan (nil if if
// hasReply is false), if send success (which is true if successfully send
// message to at least one next hop), and aggregated errors during message
// sending
func (ovl *Overlay) SendMessage(msg *protobuf.Message, routingType protobuf.RoutingType, hasReply bool) (<-chan *node.RemoteMessage, bool, error) {
	router, err := ovl.GetRouter(routingType)
	if err != nil {
		return nil, false, err
	}

	return router.SendMessage(router, &node.RemoteMessage{Msg: msg}, hasReply)
}

// SendMessageAsync sends msg to the best next hop, returns if send success
// (which is true if successfully send message to at least one next hop), and
// aggretated error during message sending
func (ovl *Overlay) SendMessageAsync(msg *protobuf.Message, routingType protobuf.RoutingType) (bool, error) {
	_, success, err := ovl.SendMessage(msg, routingType, false)
	return success, err
}

// SendMessageSync sends msg to the best next hop, returns reply message, if
// send success (which is true if successfully send message to at least one next
// hop), and aggregated error during message sending, will also returns error if
// haven't receive reply before timeout
func (ovl *Overlay) SendMessageSync(msg *protobuf.Message, routingType protobuf.RoutingType) (*protobuf.Message, bool, error) {
	replyChan, success, err := ovl.SendMessage(msg, routingType, true)
	if !success {
		return nil, success, err
	}

	select {
	case replyMsg := <-replyChan:
		return replyMsg.Msg, true, nil
	case <-time.After(replyTimeout):
		return nil, true, errors.New("Wait for reply timeout")
	}
}
