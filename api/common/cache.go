package common

import (
	"github.com/robertsarosi/rsvpn/v2/common"
	"github.com/robertsarosi/rsvpn/v2/config"
)

var rpcResultCache = common.NewGoCache(config.ConsensusDuration, config.ConsensusDuration/4)
