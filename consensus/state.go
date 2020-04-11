package consensus

import (
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/node"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/timer"
)

// startGettingNeighborConsensusState periodically checks neighbors' majority
// consensus height and sets local height if fall behind
func (consensus *Consensus) startGettingNeighborConsensusState() {
	consensus.localNode.SetMinVerifiableHeight(chain.DefaultLedger.Store.GetHeight() + por.SigChainMiningHeightOffset)

	initialized := false
	getNeighborConsensusStateTimer := time.NewTimer(proposingStartDelay / 2)
	for {
		select {
		case <-getNeighborConsensusStateTimer.C:
			majorityConsensusHeight := consensus.getNeighborsMajorityConsensusHeight()
			localConsensusHeight := consensus.GetExpectedHeight()
			localLedgerHeight := chain.DefaultLedger.Store.GetHeight()

			if !initialized {
				if majorityConsensusHeight == 0 {
					log.Infof("Cannot get neighbors' majority consensus height, assuming network bootstrap")
					consensus.localNode.SetMinVerifiableHeight(0)
				}
				initialized = true
			}

			if localConsensusHeight > majorityConsensusHeight {
				break
			}

			if localConsensusHeight == 0 || localConsensusHeight+1 < majorityConsensusHeight {
				if majorityConsensusHeight+1 > localLedgerHeight {
					consensus.setNextConsensusHeight(majorityConsensusHeight + 1)
					consensus.localNode.SetMinVerifiableHeight(majorityConsensusHeight + 1 + por.SigChainMiningHeightOffset)
					if consensus.localNode.GetSyncState() == pb.PERSIST_FINISHED {
						consensus.localNode.SetSyncState(pb.WAIT_FOR_SYNCING)
					}
				}
			}
		}
		timer.ResetTimer(getNeighborConsensusStateTimer, util.RandDuration(getConsensusStateInterval, 1.0/6.0))
	}
}

// getNeighborConsensusState returns the latest block info (height, hash, etc)
// of a neighbor using GET_CONSENSUS_STATE message
func (consensus *Consensus) getNeighborConsensusState(neighbor *node.RemoteNode) (*pb.GetConsensusStateReply, error) {
	msg, err := NewGetConsensusStateMessage()
	if err != nil {
		return nil, err
	}

	buf, err := consensus.localNode.SerializeMessage(msg, false)
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
	neighbor.SetMinVerifiableHeight(replyMsg.MinVerifiableHeight)
	neighbor.SetSyncState(replyMsg.SyncState)
	neighbor.SetLastUpdateTime(time.Now())

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
				log.Warningf("Get consensus state from neighbor %v error: %v", neighbor.GetID(), err)
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
		if i > 0 {
			time.Sleep(getConsensusStateRetryDelay)
		}

		allInfo, err := consensus.getAllNeighborsConsensusState()
		if err != nil {
			log.Warningf("Get neighbors latest block info error: %v", err)
			continue
		}

		counter := make(map[uint32]int)
		totalCount := 0
		allInfo.Range(func(key, value interface{}) bool {
			if consensusState, ok := value.(*pb.GetConsensusStateReply); ok && consensusState != nil {
				if consensusState.SyncState != pb.WAIT_FOR_SYNCING && consensusState.ConsensusHeight > 0 {
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
