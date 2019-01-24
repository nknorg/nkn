package moca

import (
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/net/node"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/timer"
)

// startGettingNeighborConsensusState peroidically checks neighbors' majority
// consensus height and sets local height if fall behind
func (consensus *Consensus) startGettingNeighborConsensusState() {
	getNeighborConsensusStateTimer := time.NewTimer(proposingStartDelay / 2)
	for {
		select {
		case <-getNeighborConsensusStateTimer.C:
			majorityConsensusHeight := consensus.getNeighborsMajorityConsensusHeight()
			localConsensusHeight := consensus.GetExpectedHeight()
			localLedgerHeight := ledger.DefaultLedger.Store.GetHeight()

			if localConsensusHeight > majorityConsensusHeight {
				break
			}

			if localConsensusHeight == 0 || localConsensusHeight+1 < majorityConsensusHeight {
				if majorityConsensusHeight+1 > localLedgerHeight {
					consensus.setNextConsensusHeight(majorityConsensusHeight + 1)
					if consensus.localNode.GetSyncState() == pb.PersistFinished {
						consensus.localNode.SetSyncState(pb.WaitForSyncing)
					}
				}
			}
		}
		timer.ResetTimer(getNeighborConsensusStateTimer, randDuration(getConsensusStateInterval, 1.0/6.0))
	}
}

// getNeighborConsensusState returns the latest block info (height, hash, etc)
// of a neighbor using GET_CONSENSUS_STATE message
func (consensus *Consensus) getNeighborConsensusState(neighbor *node.RemoteNode) (*pb.GetConsensusStateReply, error) {
	msg, err := NewGetConsensusStateMessage()
	if err != nil {
		return nil, err
	}

	buf, err := consensus.localNode.SerializeMessage(msg, true)
	if err != nil {
		return nil, err
	}

	replyBytes, err := neighbor.SendBytesSync(buf)
	if err != nil {
		return nil, err
	}

	replyMsg := &pb.GetConsensusStateReply{}
	err = proto.Unmarshal(replyBytes, replyMsg)
	if err != nil {
		return nil, err
	}

	neighbor.SetHeight(replyMsg.LedgerHeight)
	neighbor.SetSyncState(replyMsg.SyncState)

	return replyMsg, nil
}

// getAllNeighborsConsensusState returns the latest block info of all neighbors
// by calling getNeighborConsensusState on all of them concurrently.
func (consensus *Consensus) getAllNeighborsConsensusState() (*sync.Map, error) {
	var allInfo sync.Map
	var wg sync.WaitGroup
	for _, neighbor := range consensus.localNode.GetNeighbors(nil) {
		wg.Add(1)
		go func(neighbor *node.RemoteNode) {
			defer wg.Done()
			consensusState, err := consensus.getNeighborConsensusState(neighbor)
			if err != nil {
				log.Warningf("Get latest block info from neighbor %v error: %v", neighbor.GetID(), err)
				return
			}
			allInfo.Store(neighbor.GetID(), consensusState)
		}(neighbor)
	}
	wg.Wait()
	return &allInfo, nil
}

// getNeighborsMajorConsensusHeight returns the majority of neighbors' nonzero
// consensus height, or zero if no majority can be found
func (consensus *Consensus) getNeighborsMajorityConsensusHeight() uint32 {
	for i := 0; i < getConsensusStateRetries; i++ {
		time.Sleep(getConsensusStateRetryDelay)

		allInfo, err := consensus.getAllNeighborsConsensusState()
		if err != nil {
			log.Warningf("Get neighbors latest block info error: %v", err)
			continue
		}

		counter := make(map[uint32]int)
		totalCount := 0
		allInfo.Range(func(key, value interface{}) bool {
			if consensusState, ok := value.(*pb.GetConsensusStateReply); ok && consensusState != nil {
				if consensusState.SyncState != pb.WaitForSyncing && consensusState.ConsensusHeight > 0 {
					counter[consensusState.ConsensusHeight]++
					totalCount++
				}
			}
			return true
		})

		if totalCount == 0 {
			continue
		}

		for consensusHeight, count := range counter {
			if count > int(syncMinRelativeWeight*float32(totalCount)) {
				return consensusHeight
			}
		}
	}

	return 0
}
