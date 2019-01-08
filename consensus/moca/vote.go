package moca

import (
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/util/log"
)

// receiveVote is called when a vote from neighbor is received
func (consensus *Consensus) receiveVote(neighborID string, height uint32, blockHash common.Uint256) error {
	log.Debugf("Receive vote %s for height %d from neighbor %d", blockHash.ToHexString(), height, neighborID)

	if blockHash != common.EmptyUint256 {
		err := consensus.receiveProposalHash(neighborID, height, blockHash)
		if err != nil && consensus.localNode.GetSyncState() == pb.PersistFinished {
			log.Warningf("Receive block hash error when receive vote: %v", err)
		}
	}

	elc, _, err := consensus.loadOrCreateElection(heightToKey(height))
	if err != nil {
		return err
	}

	err = elc.ReceiveVote(neighborID, blockHash)
	if err != nil {
		return err
	}

	return nil
}

// vote sends out a VOTE message to all neighbors voting for a block proposal at
// certain height
func (consensus *Consensus) vote(height uint32, blockHash common.Uint256) error {
	msg, err := NewVoteMessage(height, blockHash)
	if err != nil {
		return err
	}

	buf, err := consensus.localNode.SerializeMessage(msg, true)
	if err != nil {
		return err
	}

	for _, neighbor := range consensus.localNode.GetNeighbors(nil) {
		err = neighbor.SendBytesAsync(buf)
		if err != nil {
			log.Errorf("Send vote to neighbor %v error: %v", neighbor, err)
		}
	}

	return nil
}
