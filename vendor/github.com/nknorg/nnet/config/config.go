package config

import (
	"time"

	"github.com/imdario/mergo"
)

// Config is the configuration struct
type Config struct {
	Transport             string        // which transport to use, e.g. tcp, udp, kcp
	Hostname              string        // IP or domain name for remote node to connect to, e.g. 127.0.0.1, nkn.org. Empty string means remote nodes will fill it with your address they saw, which works if all nodes are not in the same local network or are all in the local network, but will cause problem if some nodes are in the same local network
	Port                  uint16        // port to listen to incoming connections
	NodeIDBytes           uint32        // length of node id in bytes
	MinNumSuccessors      uint32        // minimal number of successors of each chord node
	NumFingerSuccessors   uint32        // minimal number of successors of each finger table key
	NumSuccessorsFactor   uint32        // number of successors is max(this factor times the number of non empty finger table, MinNumSuccessors)
	BaseStabilizeInterval time.Duration // base stabilize interval
}

// DefaultConfig returns the default configurations
func DefaultConfig() *Config {
	defaultConfig := &Config{
		Transport:             "tcp",
		NodeIDBytes:           32,
		MinNumSuccessors:      8,
		NumFingerSuccessors:   3,
		NumSuccessorsFactor:   2,
		BaseStabilizeInterval: 1 * time.Second,
	}
	return defaultConfig
}

// MergedConfig returns a new Config that use fields in conf if provided,
// otherwise use default config
func MergedConfig(conf *Config) (*Config, error) {
	merged := DefaultConfig()
	err := mergo.Merge(merged, conf, mergo.WithOverride)
	if err != nil {
		return nil, err
	}
	return merged, nil
}
