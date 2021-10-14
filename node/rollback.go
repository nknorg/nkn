package node

import (
	"errors"
	"fmt"

	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/util/log"
)

const (
	rollbackMinRelativeWeight = 1.0 / 2.0
)

func (localNode *LocalNode) maybeRollback(neighbors []*RemoteNode) (bool, error) {
	currentHeight := chain.DefaultLedger.Store.GetHeight()
	currentHash, err := chain.DefaultLedger.Store.GetBlockHash(currentHeight)
	if err != nil {
		return false, fmt.Errorf("get block hash error: %v", err)
	}

	majorityBlockHash := localNode.getNeighborsMajorityBlockHashByHeight(currentHeight, neighbors)
	if majorityBlockHash == common.EmptyUint256 {
		log.Warningf("Get neighbors majority block hash of height %d failed, skipping rollback.", currentHeight)
		return false, nil
	}

	if majorityBlockHash == currentHash {
		return false, nil
	}

	if config.Parameters.MaxRollbackBlocks == 0 {
		return false, errors.New("rollback needed but MaxRollbackBlocks is 0")
	}

	log.Warningf("Local block hash %s is different from neighbors' majority block hash %s at height %d, rollback needed", currentHash.ToHexString(), majorityBlockHash.ToHexString(), currentHeight)

	var rollbackToHeight uint32
	if currentHeight > config.Parameters.MaxRollbackBlocks {
		rollbackToHeight = currentHeight - config.Parameters.MaxRollbackBlocks
	}

	majorityBlockHash = localNode.getNeighborsMajorityBlockHashByHeight(rollbackToHeight, neighbors)
	if majorityBlockHash == common.EmptyUint256 {
		return false, fmt.Errorf("get neighbors majority block hash at rollback height failed")
	}

	rollbackHash, _ := chain.DefaultLedger.Store.GetBlockHash(rollbackToHeight)
	if majorityBlockHash != rollbackHash {
		return false, fmt.Errorf("local ledger has forked for more than %d blocks", config.Parameters.MaxRollbackBlocks)
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
