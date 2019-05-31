package node

import (
	"fmt"
	"sync"
	"time"

	"github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

const (
	rollbackRetries           = 3
	rollbackRetryDelay        = 3 * time.Second
	rollbackMinRelativeWeight = 1.0 / 2.0
)

func (localNode *LocalNode) maybeRollback(neighbors []*RemoteNode) (bool, error) {
	currentHeight := chain.DefaultLedger.Store.GetHeight()
	currentHash, _ := chain.DefaultLedger.Store.GetBlockHash(currentHeight)

	majorityBlockHash := localNode.getNeighborsMajorityBlockHashByHeight(currentHeight, neighbors)
	if majorityBlockHash == common.EmptyUint256 {
		log.Warningf("Get neighbors majority block hash by height failed")
		return false, nil
	}

	if majorityBlockHash != currentHash {
		err := fmt.Errorf("Local block hash %s is different from neighbors' majority block hash %s at height %d, rollback needed", currentHash.ToHexString(), majorityBlockHash.ToHexString(), currentHeight)
		if config.MaxRollbackBlocks == 0 {
			return false, err
		}
		log.Warning(err)
	} else {
		return false, nil
	}

	var rollbackToHeight uint32
	if currentHeight > config.MaxRollbackBlocks {
		rollbackToHeight = currentHeight - config.MaxRollbackBlocks
	} else {
		rollbackToHeight = 0
	}

	majorityBlockHash = localNode.getNeighborsMajorityBlockHashByHeight(rollbackToHeight, neighbors)
	if majorityBlockHash == common.EmptyUint256 {
		return false, fmt.Errorf("get neighbors majority block hash at rollback height failed")
	}

	rollbackHash, _ := chain.DefaultLedger.Store.GetBlockHash(rollbackToHeight)
	if majorityBlockHash != rollbackHash {
		return false, fmt.Errorf("local ledger has forked for more than %d blocks", config.MaxRollbackBlocks)
	}

	for rollbackHeight := currentHeight; rollbackHeight > rollbackToHeight; rollbackHeight-- {
		block, err := chain.DefaultLedger.Store.GetBlockByHeight(rollbackHeight)
		if err != nil {
			return false, fmt.Errorf("get block at height %d error: %v", rollbackHeight, err)
		}
		err = chain.DefaultLedger.Store.Rollback(block)
		if err != nil {
			return false, fmt.Errorf("ledger rollback error: %v", err)
		}
	}

	log.Infof("Rollback to block height %d", rollbackToHeight)

	return true, nil
}

// getNeighborsBlockHeaderByHeight returns the block header at a given height
// from given neighbors by calling GetBlockHeaders on all of them concurrently.
func (localNode *LocalNode) getNeighborsBlockHeaderByHeight(height uint32, neighbors []*RemoteNode) (*sync.Map, error) {
	var allHeaders sync.Map
	var wg sync.WaitGroup
	for _, neighbor := range neighbors {
		wg.Add(1)
		go func(neighbor *RemoteNode) {
			defer wg.Done()
			headers, err := neighbor.GetBlockHeaders(height, height)
			if err != nil {
				log.Warningf("Get block header at height %d from neighbor %v error: %v", height, neighbor.GetID(), err)
				return
			}
			allHeaders.Store(neighbor.GetID(), headers[0])
		}(neighbor)
	}
	wg.Wait()
	return &allHeaders, nil
}

// getNeighborsMajorityBlockHashByHeight returns the majority of given
// neighbors' block hash at a give height
func (localNode *LocalNode) getNeighborsMajorityBlockHashByHeight(height uint32, neighbors []*RemoteNode) common.Uint256 {
	for i := 0; i < rollbackRetries; i++ {
		time.Sleep(rollbackRetryDelay)

		allHeaders, err := localNode.getNeighborsBlockHeaderByHeight(height, neighbors)
		if err != nil {
			log.Warningf("Get neighbors block header at height %d error: %v", height, err)
			continue
		}

		counter := make(map[common.Uint256]int)
		totalCount := 0
		allHeaders.Range(func(key, value interface{}) bool {
			if header, ok := value.(*block.Header); ok && header != nil {
				counter[header.Hash()]++
				totalCount++
			}
			return true
		})

		if totalCount == 0 {
			continue
		}

		for blockHash, count := range counter {
			if count > int(rollbackMinRelativeWeight*float32(totalCount)) {
				return blockHash
			}
		}
	}

	return common.EmptyUint256
}
