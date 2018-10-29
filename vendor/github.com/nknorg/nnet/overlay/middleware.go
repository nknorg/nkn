package overlay

// NetworkWillStart is called right before network starts listening and handling
// messages. It can be used to do additional network setup like NAT traversal.
type NetworkWillStart func(Network) bool

// NetworkStarted is called right after network starts listening and handling
// messages.
type NetworkStarted func(Network) bool

// NetworkWillStop is called right before network stops listening and handling
// messages.
type NetworkWillStop func(Network) bool

// NetworkStopped is called right after network stops listening and handling
// messages. It can be used to do some clean up, etc.
type NetworkStopped func(Network) bool
