package ising

import (
	"errors"
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/util/log"
)

const (
	MinPingRespNum = 3
)

type HeightHash map[uint32]*Uint256
type NodersPong map[uint64]HeightHash

type ForkCache struct {
	sync.RWMutex
	currentHash    Uint256    // current hash when send ping
	currentHeight  uint32     // current height when send ping
	pingResponse   NodersPong // cache ping response for each neighbor
	probeHeight    uint32     // which height want to ping for detecting fork
	probeResponse  NodersPong // cache detection response for each neighbor
	rollBackHeight uint32     // the height which will roll back
}

func NewForkCache(height uint32, hash Uint256) *ForkCache {
	cache := &ForkCache{
		currentHeight:  height,
		currentHash:    hash,
		pingResponse:   make(NodersPong),
		probeHeight:    0,
		probeResponse:  make(NodersPong),
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
	fc.pingResponse[sender] = HeightHash{height: &hash}

	return nil
}

// AnalyzePingResp: return true when my hash different with majority of neighbors
func (fc *ForkCache) AnalyzePingResp() bool {
	fc.RLock()
	defer fc.RUnlock()

	bestHash, bestHeight, _, ok := fc.handleResponse(fc.pingResponse)
	if !ok {
		return false
	}

	hash, err := ledger.DefaultLedger.Store.GetBlockHash(bestHeight)
	if err != nil {
		return true
	}

	return hash.CompareTo(bestHash) != 0
}

func (fc *ForkCache) CacheProbeResp(hash Uint256, height uint32, sender uint64) error {
	fc.Lock()
	defer fc.Unlock()

	if _, ok := fc.probeResponse[sender]; ok {
		return errors.New("receive duplicated pong message in probe ping")
	}
	fc.probeResponse[sender] = HeightHash{height: &hash}

	return nil
}

func (fc *ForkCache) AnalyzeProbeResp() bool {
	fc.Lock()
	defer fc.Unlock()

	bestHash, bestHeight, _, ok := fc.handleResponse(fc.probeResponse)
	if !ok {
		return false
	}
	if bestHeight != fc.probeHeight {
		log.Errorf("PongResp Height[%v] NOT the appointed height[%v]", bestHeight, fc.probeResponse)
		// FIXME: Maybe my Chain higher than majority neighbors
	}
	hash, _ := ledger.DefaultLedger.Store.GetBlockHash(fc.probeHeight)
	if hash.CompareTo(bestHash) != 0 {
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

type HashSustainer struct {
	height uint32
	nids   []uint64
}

func (fc *ForkCache) handleResponse(resp NodersPong) (Uint256, uint32, []uint64, bool) {
	total := len(resp)
	if total >= MinPingRespNum {
		n := make(map[Uint256]*HashSustainer)
		for nid, tuple := range resp {
			for height, hash := range tuple {
				if stat, ok := n[*hash]; ok {
					if stat.height != height {
						log.Errorf("Wow! So Lucky! You met a HASH confliction "+
							"in case %x (%v, %v) vs (%v, %v)",
							hash, nid, height, stat.height, stat.nids)
					}
					stat.nids = append(stat.nids, nid)
				} else {
					n[*hash] = &HashSustainer{height, []uint64{nid}}
				}
			}
		}
		for hash, stat := range n {
			// log.Debugf("%x:%d %v", hash, len(stat.nids), stat.nids)
			if 2*len(stat.nids) > total {
				return hash, stat.height, stat.nids, true
			}
		}
		log.Warningf("Calc %d PongResp but no majority faction", total)
	}
	return EmptyUint256, 0, nil, false
}
