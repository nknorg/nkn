package common

import (
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
)

var rpcResultCache = common.NewGoCache(config.ConsensusDuration, config.ConsensusDuration/4)
