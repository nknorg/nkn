package consensus

import (
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/node"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/por"
	"github.com/nknorg/nkn/v2/util"
	"github.com/nknorg/nkn/v2/util/log"
	"github.com/nknorg/nkn/v2/util/timer"
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
			majorityConsensusHeight, majorityLedgerHeight, majorityLedgerBlockHash := consensus.getNeighborsMajorityConsensusState()
			localConsensusHeight := consensus.GetExpectedHeight()
			localLedgerHeight := chain.DefaultLedger.Store.GetHeight()
			localLedgerBlockHash := chain.DefaultLedger.Store.GetHeaderHashByHeight(localLedgerHeight)

			if !initialized {
				if majorityConsensusHeight == 0 {
					log.Infof("Cannot get neighbors' majority consensus height, assuming network bootstrap.")
					consensus.localNode.SetMinVerifiableHeight(0)
				}
				initialized = true
			}

			if majorityLedgerHeight > 0 && majorityLedgerBlockHash != common.EmptyUint256 {
				if localLedgerHeight == majorityLedgerHeight && localLedgerBlockHash != majorityLedgerBlockHash {
					log.Infof("Latest local block is different from neighbors' majority.")
					// Increase local consensus height by 3 to avoid next consensus round
					// overlap with current round
					consensus.setNextConsensusHeight(localConsensusHeight + 3)
					consensus.localNode.SetMinVerifiableHeight(localConsensusHeight + 3 + por.SigChainMiningHeightOffset)
					if consensus.localNode.GetSyncState() == pb.SyncState_PERSIST_FINISHED {
						consensus.localNode.SetSyncState(pb.SyncState_WAIT_FOR_SYNCING)
					}
				}
			}

			if localConsensusHeight == 0 || localConsensusHeight+1 < majorityConsensusHeight {
				if majorityConsensusHeight+1 > localLedgerHeight {
					log.Infof("Local consensus height fall behind from neighbors' majority.")
					// Increase local consensus height by at least 3 for the same reason
					// above
					consensus.setNextConsensusHeight(majorityConsensusHeight + 1)
					consensus.localNode.SetMinVerifiableHeight(majorityConsensusHeight + 1 + por.SigChainMiningHeightOffset)
					if consensus.localNode.GetSyncState() == pb.SyncState_PERSIST_FINISHED {
						consensus.localNode.SetSyncState(pb.SyncState_WAIT_FOR_SYNCING)
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

// getVotingNeighborsConsensusState returns the latest block info of voting
// neighbors by calling getNeighborConsensusState on all of them concurrently.
// It will also update the consensus state of non-voting neighbors.
func (consensus *Consensus) getVotingNeighborsConsensusState() (map[string]*pb.GetConsensusStateReply, error) {
	allNeighbors := consensus.localNode.GetNeighbors(nil)
	votingNeighbors := consensus.localNode.GetVotingNeighbors(nil)
	if len(votingNeighbors) < minConsensusStateNeighbors {
		votingNeighbors = allNeighbors
	}
	votingNeighborsMap := make(map[string]struct{}, len(votingNeighbors))
	for _, neighbor := range votingNeighbors {
		votingNeighborsMap[neighbor.GetID()] = struct{}{}
	}

	states := make(map[string]*pb.GetConsensusStateReply, len(votingNeighbors))
	var wg sync.WaitGroup
	var lock sync.Mutex
	for _, neighbor := range allNeighbors {
		wg.Add(1)
		go func(neighbor *node.RemoteNode) {
			defer wg.Done()
			consensusState, err := consensus.getNeighborConsensusState(neighbor)
			if err != nil {
				log.Debugf("Get consensus state from neighbor %v error: %v", neighbor.GetID(), err)
				return
			}
			neighborID := neighbor.GetID()
			if _, ok := votingNeighborsMap[neighborID]; ok {
				lock.Lock()
				states[neighborID] = consensusState
				lock.Unlock()
			}
		}(neighbor)
	}
	wg.Wait()
	return states, nil
}

// getNeighborsMajorConsensusHeight returns the majority of neighbors' nonzero
// consensus height, ledger height, ledger block hash, or empty value if no
// majority can be found
func (consensus *Consensus) getNeighborsMajorityConsensusState() (uint32, uint32, common.Uint256) {
	for i := 0; i < getConsensusStateRetries; i++ {
		if i > 0 {
			time.Sleep(getConsensusStateRetryDelay)
		}

		allStates, err := consensus.getVotingNeighborsConsensusState()
		if err != nil {
			log.Warningf("Get neighbors latest block info error: %v", err)
			continue
		}

		consensusHeightCount := make(map[uint32]int)
		ledgerHeightCount := make(map[uint32]int)
		ledgerBlockHashCount := make(map[common.Uint256]int)
		var consensusHeightTotalCount, ledgerHeightTotalCount, ledgerBlockHashTotalCount int
		for _, consensusState := range allStates {
			if consensusState.SyncState != pb.SyncState_WAIT_FOR_SYNCING && consensusState.ConsensusHeight > 0 {
				consensusHeightCount[consensusState.ConsensusHeight]++
				consensusHeightTotalCount++
			}

			if consensusState.SyncState == pb.SyncState_PERSIST_FINISHED && consensusState.LedgerHeight > 0 {
				ledgerHeightCount[consensusState.LedgerHeight]++
				ledgerHeightTotalCount++
			}

			if consensusState.SyncState == pb.SyncState_PERSIST_FINISHED {
				hash, err := common.Uint256ParseFromBytes(consensusState.LedgerBlockHash)
				if err == nil && hash != common.EmptyUint256 {
					ledgerBlockHashCount[hash]++
					ledgerBlockHashTotalCount++
				}
			}
		}

		var majorityConsensusHeight uint32
		for consensusHeight, count := range consensusHeightCount {
			if count > int(syncMinRelativeWeight*float32(consensusHeightTotalCount)) {
				majorityConsensusHeight = consensusHeight
				break
			}
		}

		var majorityLedgerHeight uint32
		for ledgerHeight, count := range ledgerHeightCount {
			if count > int(syncMinRelativeWeight*float32(ledgerHeightTotalCount)) {
				majorityLedgerHeight = ledgerHeight
				break
			}
		}

		var majorityLedgerBlockHash common.Uint256
		for blockHash, count := range ledgerBlockHashCount {
			if count > int(syncMinRelativeWeight*float32(ledgerBlockHashTotalCount)) {
				majorityLedgerBlockHash = blockHash
				break
			}
		}

		if majorityConsensusHeight > 0 || majorityLedgerHeight > 0 || majorityLedgerBlockHash != common.EmptyUint256 {
			return majorityConsensusHeight, majorityLedgerHeight, majorityLedgerBlockHash
		}
	}

	return 0, 0, common.EmptyUint256
}
