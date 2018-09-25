package ising

import (
	"errors"
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
)

const (
	MinPingRespNum = 3
)

type ForkCache struct {
	sync.RWMutex
	currentHash    Uint256             // current hash when send ping
	currentHeight  uint32              // current height when send ping
	pingResponse   map[uint64]*Uint256 // cache ping response for each neighbor
	probeHeight    uint32              // which height want to ping for detecting fork
	probeResponse  map[uint64]*Uint256 // cache detection response for each neighbor
	rollBackHeight uint32              // the height which will roll back
}

func NewForkCache(height uint32, hash Uint256) *ForkCache {
	cache := &ForkCache{
		currentHeight:  height,
		currentHash:    hash,
		pingResponse:   make(map[uint64]*Uint256),
		probeHeight:    0,
		probeResponse:  make(map[uint64]*Uint256),
		rollBackHeight: 0,
	}

	return cache
}

func (fc *ForkCache) CachePingResp(hash Uint256, height uint32, sender uint64) error {
	fc.Lock()
	defer fc.Unlock()

	if _, ok := fc.pingResponse[sender]; ok {
		return errors.New("receive duplicated pong message in regular ping")
	}
	fc.pingResponse[sender] = &hash

	return nil
}

func (fc *ForkCache) AnalyzePingResp() bool {
	fc.RLock()
	defer fc.RUnlock()

	bestHash, ok := fc.handleResponse(fc.pingResponse)
	if !ok {
		return false
	}
	if fc.currentHash.CompareTo(*bestHash) == 0 {
		return false
	}

	return true
}

func (fc *ForkCache) CacheProbeResp(hash Uint256, height uint32, sender uint64) error {
	fc.Lock()
	defer fc.Unlock()

	fc.probeResponse[sender] = &hash

	return nil
}

func (fc *ForkCache) AnalyzeProbeResp() bool {
	fc.Lock()
	defer fc.Unlock()

	bestHash, ok := fc.handleResponse(fc.probeResponse)
	if !ok {
		return false
	}
	hash, _ := ledger.DefaultLedger.Store.GetBlockHash(fc.probeHeight)
	if hash.CompareTo(*bestHash) != 0 {
		return false
	}
	fc.SetRollBackHeight(fc.probeHeight)

	return true
}

func (fc *ForkCache) GetCurrentHeight() uint32 {
	return fc.currentHeight
}

func (fc *ForkCache) SetRollBackHeight(height uint32) {
	fc.rollBackHeight = height
}

func (fc *ForkCache) GetRollBackHeight() uint32 {
	return fc.rollBackHeight
}

func (fc *ForkCache) handleResponse(resp map[uint64]*Uint256) (*Uint256, bool) {
	total := len(resp)
	if total < MinPingRespNum {
		return nil, false
	}
	n := make(map[Uint256]int)
	for _, hash := range resp {
		n[*hash]++
	}
	var bestHash *Uint256
	for hash, count := range n {
		if 2*count > total {
			bestHash = &hash
			break
		}
	}
	if bestHash == nil {
		return nil, false
	}

	return bestHash, true
}
