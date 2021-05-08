package por

import (
	"sync"
	"time"

	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
)

var (
	recentMinerLock  sync.Mutex
	recentMinerCache common.Cache
)

type RecentMiner map[string]int

func init() {
	recentMinerCache = common.NewGoCache(5*time.Minute, time.Minute)
}

func GetRecentMiner(blockHash []byte) (RecentMiner, error) {
	if v, ok := recentMinerCache.Get(blockHash); ok {
		if rm, ok := v.(RecentMiner); ok {
			return rm, nil
		}
	}

	recentMinerLock.Lock()
	defer recentMinerLock.Unlock()

	if v, ok := recentMinerCache.Get(blockHash); ok {
		if rm, ok := v.(RecentMiner); ok {
			return rm, nil
		}
	}

	hash := blockHash
	rm := make(RecentMiner, config.SigChainRecentMinerBlocks)
	for i := 0; i < config.SigChainRecentMinerBlocks; i++ {
		hashUint256, err := common.Uint256ParseFromBytes(hash)
		if err != nil {
			return nil, err
		}
		header, err := store.GetHeaderWithCache(hashUint256)
		if err != nil {
			return nil, err
		}
		rm[string(header.UnsignedHeader.SignerPk)]++
		hash = header.UnsignedHeader.PrevBlockHash
	}

	recentMinerCache.Set(blockHash, rm)

	return rm, nil
}
