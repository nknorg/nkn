package nnet

import (
	"github.com/nknorg/nnet/config"
	"github.com/nknorg/nnet/log"
	"github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay"
	"github.com/nknorg/nnet/overlay/chord"
	"github.com/nknorg/nnet/util"
)

// NNet is is a peer to peer network
type NNet struct {
	overlay.Network
	Config *config.Config
}

// Config is an alias of config.Config for simpler usage
type Config config.Config

// NewNNet creates a new nnet using the local node id and configuration
// provided. If id is nil, a random id will be generated. If conf is nil, the
// default config will be used.
func NewNNet(id []byte, conf *Config) (*NNet, error) {
	var mergedConf *config.Config
	var err error

	if conf != nil {
		convertedConf := config.Config(*conf)
		mergedConf, err = config.MergedConfig(&convertedConf)
		if err != nil {
			return nil, err
		}
	} else {
		mergedConf = config.DefaultConfig()
	}

	if len(id) == 0 {
		id, err = util.RandBytes(int(mergedConf.NodeIDBytes))
		if err != nil {
			return nil, err
		}
	}

	localNode, err := node.NewLocalNode(id[:], mergedConf)
	if err != nil {
		return nil, err
	}

	network, err := chord.NewChord(localNode, mergedConf)
	if err != nil {
		return nil, err
	}

	nn := &NNet{
		Network: network,
		Config:  mergedConf,
	}

	return nn, nil
}

// SetLogger sets the global logger
func SetLogger(logger log.Logger) error {
	return log.SetLogger(logger)
}
