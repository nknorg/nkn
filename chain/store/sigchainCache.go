package store

import (
	"errors"
	"sync"

	"github.com/nknorg/nkn/v2/block"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/pb"
)

type SigChainCache struct {
	mu            sync.RWMutex
	sigChainCache map[common.Uint256]*pb.SigChain
}

func NewSigChainCache() *SigChainCache {
	return &SigChainCache{
		sigChainCache: make(map[common.Uint256]*pb.SigChain),
	}
}

func (s *SigChainCache) AddSigChainToCache(hash common.Uint256, sc *pb.SigChain) {
	s.mu.Lock()
	s.sigChainCache[hash] = sc
	s.mu.Unlock()
}

func (s *SigChainCache) RemoveCachedSigChain(h []byte) {
	hash, err := common.Uint256ParseFromBytes(h)
	if err == nil && hash != common.EmptyUint256 {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.sigChainCache, hash)
	}
}

func (s *SigChainCache) GetCachedSigChain(hash common.Uint256) (*pb.SigChain, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sc, ok := s.sigChainCache[hash]; !ok {
		return nil, errors.New("no sigChain in cache")
	} else {
		return sc, nil
	}
}

func (s *SigChainCache) RollbackSigChain(h *block.Header) {
	s.RemoveCachedSigChain(h.UnsignedHeader.WinnerHash)
}
