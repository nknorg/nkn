package por

import (
	"sort"
	"sync"

	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nnet/overlay/chord"
)

var (
	recentMinerLock  sync.Mutex
	recentMinerCache common.Cache
	skipMinerLock    sync.Mutex
	skipMinerCache   common.Cache
)

type RecentMiner map[string]int
type SkipMiner [][]byte

func init() {
	recentMinerCache = common.NewGoCache(10*config.ConsensusDuration, config.ConsensusDuration)
	skipMinerCache = common.NewGoCache(10*config.ConsensusDuration, config.ConsensusDuration)
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
		if header.UnsignedHeader.Height == 0 {
			break
		}
		rm[string(header.UnsignedHeader.SignerPk)]++
		hash = header.UnsignedHeader.PrevBlockHash
	}

	recentMinerCache.Set(blockHash, rm)

	return rm, nil
}

func GetSkipMiner(blockHash []byte) (SkipMiner, error) {
	if v, ok := skipMinerCache.Get(blockHash); ok {
		if sm, ok := v.(SkipMiner); ok {
			return sm, nil
		}
	}

	skipMinerLock.Lock()
	defer skipMinerLock.Unlock()

	if v, ok := skipMinerCache.Get(blockHash); ok {
		if sm, ok := v.(SkipMiner); ok {
			return sm, nil
		}
	}

	hash := blockHash
	sm := make(SkipMiner, 0, config.SigChainSkipMinerBlocks)
	minerSet := make(map[common.Uint256]struct{}, config.SigChainSkipMinerBlocks)
	for i := 0; i < config.SigChainSkipMinerBlocks; i++ {
		hashUint256, err := common.Uint256ParseFromBytes(hash)
		if err != nil {
			return nil, err
		}
		header, err := store.GetHeaderWithCache(hashUint256)
		if err != nil {
			return nil, err
		}
		if header.UnsignedHeader.Height == 0 {
			break
		}
		winnerHash, err := common.Uint256ParseFromBytes(header.UnsignedHeader.WinnerHash)
		if err != nil {
			return nil, err
		}
		if winnerHash != common.EmptyUint256 {
			sc, err := store.GetSigChainWithCache(winnerHash)
			if err != nil {
				return nil, err
			}
			for i := 1; i < sc.Length()-1; i++ {
				id, err := common.Uint256ParseFromBytes(sc.Elems[i].Id)
				if err != nil {
					return nil, err
				}
				if _, ok := minerSet[id]; !ok {
					minerSet[id] = struct{}{}
					sm = append(sm, sc.Elems[i].Id)
				}
			}
		}
		hash = header.UnsignedHeader.PrevBlockHash
	}

	sort.Slice(sm, func(i int, j int) bool {
		return chord.CompareID(sm[i], sm[j]) < 0
	})

	skipMinerCache.Set(blockHash, sm)

	return sm, nil
}
