package node

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core"
	"github.com/nknorg/nkn/net/node/consequential"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/types"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

const (
	concurrentSyncRequestPerNeighbor = 1
	maxSyncBlockHeadersBatchSize     = 1024
	maxSyncBlocksBatchSize           = 32
	syncReplyTimeout                 = 20 * time.Second
	maxSyncWorkerFails               = 3
	syncWorkerStartInterval          = 20 * time.Millisecond
)

// NewGetBlockHeadersMessage creates a GET_BLOCK_HEADERS message
func NewGetBlockHeadersMessage(startHeight, endHeight uint32) (*pb.UnsignedMessage, error) {
	msgBody := &pb.GetBlockHeaders{
		StartHeight: startHeight,
		EndHeight:   endHeight,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.GET_BLOCK_HEADERS,
		Message:     buf,
	}

	return msg, nil
}

// NewGetBlockHeadersReply creates a GET_BLOCK_HEADERS_REPLY message in respond
// to GET_BLOCK_HEADERS message
func NewGetBlockHeadersReply(headers []*types.Header) (*pb.UnsignedMessage, error) {
	headersBytes := make([][]byte, len(headers), len(headers))
	for i, header := range headers {
		b := new(bytes.Buffer)
		err := header.Serialize(b)
		if err != nil {
			return nil, err
		}
		headersBytes[i] = b.Bytes()
	}

	msgBody := &pb.GetBlockHeadersReply{
		BlockHeaders: headersBytes,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.GET_BLOCK_HEADERS_REPLY,
		Message:     buf,
	}

	return msg, nil
}

// NewGetBlocksMessage creates a GET_BLOCKS message
func NewGetBlocksMessage(startHeight, endHeight uint32) (*pb.UnsignedMessage, error) {
	msgBody := &pb.GetBlocks{
		StartHeight: startHeight,
		EndHeight:   endHeight,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.GET_BLOCKS,
		Message:     buf,
	}

	return msg, nil
}

// NewGetBlocksReply creates a GET_BLOCKS_REPLY message in respond to GET_BLOCKS
// message
func NewGetBlocksReply(blocks []*types.Block) (*pb.UnsignedMessage, error) {
	blocksBytes := make([][]byte, len(blocks), len(blocks))
	for i, block := range blocks {
		b := new(bytes.Buffer)
		err := block.Serialize(b)
		if err != nil {
			return nil, err
		}
		blocksBytes[i] = b.Bytes()
	}

	msgBody := &pb.GetBlocksReply{
		Blocks: blocksBytes,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.GET_BLOCKS_REPLY,
		Message:     buf,
	}

	return msg, nil
}

// getBlockHeadersMessageHandler handles a GET_BLOCK_HEADERS message
func (localNode *LocalNode) getBlockHeadersMessageHandler(remoteMessage *RemoteMessage) ([]byte, bool, error) {
	replyMsg, err := NewGetBlockHeadersReply(nil)
	if err != nil {
		return nil, false, err
	}

	replyBuf, err := localNode.SerializeMessage(replyMsg, false)
	if err != nil {
		return nil, false, err
	}

	msgBody := &pb.GetBlockHeaders{}
	err = proto.Unmarshal(remoteMessage.Message, msgBody)
	if err != nil {
		return replyBuf, false, err
	}

	startHeight, endHeight := msgBody.StartHeight, msgBody.EndHeight

	if endHeight < startHeight {
		return replyBuf, false, nil
	}

	if endHeight-startHeight > maxSyncBlockHeadersBatchSize {
		return replyBuf, false, nil
	}

	if endHeight > core.DefaultLedger.Store.GetHeaderHeight() {
		return replyBuf, false, nil
	}

	headers := make([]*types.Header, endHeight-startHeight+1, endHeight-startHeight+1)
	for height := startHeight; height <= endHeight; height++ {
		headers[height-startHeight], err = core.DefaultLedger.Store.GetHeaderByHeight(height)
		if err != nil {
			return replyBuf, false, err
		}
	}

	replyMsg, err = NewGetBlockHeadersReply(headers)
	if err != nil {
		return replyBuf, false, err
	}

	replyBuf, err = localNode.SerializeMessage(replyMsg, false)
	return replyBuf, false, err
}

// getBlocksMessageHandler handles a GET_BLOCKS message
func (localNode *LocalNode) getBlocksMessageHandler(remoteMessage *RemoteMessage) ([]byte, bool, error) {
	replyMsg, err := NewGetBlocksReply(nil)
	if err != nil {
		return nil, false, err
	}

	replyBuf, err := localNode.SerializeMessage(replyMsg, false)
	if err != nil {
		return nil, false, err
	}

	msgBody := &pb.GetBlocks{}
	err = proto.Unmarshal(remoteMessage.Message, msgBody)
	if err != nil {
		return replyBuf, false, err
	}

	startHeight, endHeight := msgBody.StartHeight, msgBody.EndHeight

	if endHeight < startHeight {
		return replyBuf, false, nil
	}

	if endHeight-startHeight > maxSyncBlocksBatchSize {
		return replyBuf, false, nil
	}

	if endHeight > core.DefaultLedger.Store.GetHeight() {
		return replyBuf, false, nil
	}

	blocks := make([]*types.Block, endHeight-startHeight+1, endHeight-startHeight+1)
	for height := startHeight; height <= endHeight; height++ {
		blocks[height-startHeight], err = core.DefaultLedger.Store.GetBlockByHeight(height)
		if err != nil {
			return replyBuf, false, err
		}
	}

	replyMsg, err = NewGetBlocksReply(blocks)
	if err != nil {
		return replyBuf, false, err
	}

	replyBuf, err = localNode.SerializeMessage(replyMsg, false)
	return replyBuf, false, err
}

// GetBlockHeaders requests a range of consecutive block headers from a neighbor
// using GET_BLOCK_HEADERS message
func (remoteNode *RemoteNode) GetBlockHeaders(startHeight, endHeight uint32) ([]*types.Header, error) {
	if startHeight > endHeight {
		return nil, fmt.Errorf("start height %d is higher than end height %d", startHeight, endHeight)
	}

	msg, err := NewGetBlockHeadersMessage(startHeight, endHeight)
	if err != nil {
		return nil, err
	}

	buf, err := remoteNode.localNode.SerializeMessage(msg, true)
	if err != nil {
		return nil, err
	}

	replyBytes, err := remoteNode.SendBytesSyncWithTimeout(buf, syncReplyTimeout)
	if err != nil {
		return nil, err
	}

	replyMsg := &pb.GetBlockHeadersReply{}
	err = proto.Unmarshal(replyBytes, replyMsg)
	if err != nil {
		return nil, err
	}

	if len(replyMsg.BlockHeaders) < int(endHeight-startHeight+1) {
		return nil, fmt.Errorf("result contains %d instead of %d headers", len(replyMsg.BlockHeaders), endHeight-startHeight+1)
	}

	headers := make([]*types.Header, len(replyMsg.BlockHeaders), len(replyMsg.BlockHeaders))
	for i := range replyMsg.BlockHeaders {
		headers[i] = &types.Header{}
		err = headers[i].Deserialize(bytes.NewReader(replyMsg.BlockHeaders[i]))
		if err != nil {
			return nil, err
		}
	}

	return headers, nil
}

// GetBlocks requests a range of consecutive blocks from a neighbor using
// GET_BLOCKS message
func (remoteNode *RemoteNode) GetBlocks(startHeight, endHeight uint32) ([]*types.Block, error) {
	if startHeight > endHeight {
		return nil, fmt.Errorf("start height %d is higher than end height %d", startHeight, endHeight)
	}

	msg, err := NewGetBlocksMessage(startHeight, endHeight)
	if err != nil {
		return nil, err
	}

	buf, err := remoteNode.localNode.SerializeMessage(msg, true)
	if err != nil {
		return nil, err
	}

	replyBytes, err := remoteNode.SendBytesSyncWithTimeout(buf, syncReplyTimeout)
	if err != nil {
		return nil, err
	}

	replyMsg := &pb.GetBlocksReply{}
	err = proto.Unmarshal(replyBytes, replyMsg)
	if err != nil {
		return nil, err
	}

	if len(replyMsg.Blocks) < int(endHeight-startHeight+1) {
		return nil, fmt.Errorf("result contains %d instead of %d blocks", len(replyMsg.Blocks), endHeight-startHeight+1)
	}

	blocks := make([]*types.Block, len(replyMsg.Blocks), len(replyMsg.Blocks))
	for i := range replyMsg.Blocks {
		blocks[i] = &types.Block{}
		err = blocks[i].Deserialize(bytes.NewReader(replyMsg.Blocks[i]))
		if err != nil {
			return nil, err
		}
	}

	return blocks, nil
}

// initSyncing initializes block syncing state and registers message handler
func (localNode *LocalNode) initSyncing() {
	localNode.AddMessageHandler(pb.GET_BLOCK_HEADERS, localNode.getBlockHeadersMessageHandler)
	localNode.AddMessageHandler(pb.GET_BLOCKS, localNode.getBlocksMessageHandler)
	localNode.ResetSyncing()
}

// StartSyncing starts block syncing from current local ledger until it gets to
// block height stopHeight with block hash stopHash from given neighbors
func (localNode *LocalNode) StartSyncing(stopHash common.Uint256, stopHeight uint32, neighbors []*RemoteNode) (bool, error) {
	var err error
	started := false

	localNode.RLock()
	syncOnce := localNode.syncOnce
	localNode.RUnlock()

	syncOnce.Do(func() {
		started = true
		localNode.SetSyncState(pb.SyncStarted)

		currentHeight := core.DefaultLedger.Store.GetHeight()
		currentHash := core.DefaultLedger.Store.GetHeaderHashByHeight(currentHeight)
		if stopHeight <= currentHeight {
			err = fmt.Errorf("sync stop height %d is not higher than current height %d", stopHeight, currentHeight)
			return
		}

		if len(neighbors) == 0 {
			err = fmt.Errorf("no neighbors to sync from")
			return
		}

		var rollbacked bool
		rollbacked, err = localNode.maybeRollback(neighbors)
		if err != nil {
			panic(fmt.Errorf("Rollback error: %v", err))
		}
		if rollbacked {
			currentHeight = core.DefaultLedger.Store.GetHeight()
			currentHash = core.DefaultLedger.Store.GetHeaderHashByHeight(currentHeight)
		}

		startTime := time.Now()
		var headersHash []common.Uint256
		headersHash, err = localNode.syncBlockHeaders(currentHeight+1, stopHeight, currentHash, stopHash, neighbors)
		if err != nil {
			err = fmt.Errorf("sync block headers error: %v", err)
			return
		}

		log.Infof("Synced %d block headers in %s", stopHeight-currentHeight, time.Since(startTime))

		startTime = time.Now()
		err = localNode.syncBlocks(currentHeight+1, stopHeight, neighbors, headersHash)
		if err != nil {
			err = fmt.Errorf("sync blocks error: %v", err)
			return
		}

		log.Infof("Synced %d blocks in %s", stopHeight-currentHeight, time.Since(startTime))

		localNode.SetSyncState(pb.SyncFinished)
	})

	return started, err
}

// ResetSyncing resets syncOnce and allows for future block syncing
func (localNode *LocalNode) ResetSyncing() {
	localNode.Lock()
	defer localNode.Unlock()
	localNode.syncOnce = new(sync.Once)
}

func (localNode *LocalNode) syncBlockHeaders(startHeight, stopHeight uint32, startPrevHash, stopHash common.Uint256, neighbors []*RemoteNode) ([]common.Uint256, error) {
	var nextHeader *types.Header
	headersHash := make([]common.Uint256, stopHeight-startHeight+1, stopHeight-startHeight+1)
	numBatches := (stopHeight-startHeight)/config.Parameters.SyncBlockHeadersBatchSize + 1
	numWorkers := uint32(len(neighbors)) * concurrentSyncRequestPerNeighbor

	getBatchHeightRange := func(batchID uint32) (uint32, uint32) {
		batchStartHeight := startHeight + batchID*config.Parameters.SyncBlockHeadersBatchSize
		batchEndHeight := batchStartHeight + config.Parameters.SyncBlockHeadersBatchSize - 1
		if batchEndHeight > stopHeight {
			batchEndHeight = stopHeight
		}
		return batchStartHeight, batchEndHeight
	}

	getHeader := func(workerID, batchID uint32) (interface{}, bool) {
		neighbor := neighbors[workerID%uint32(len(neighbors))]
		batchStartHeight, batchEndHeight := getBatchHeightRange(batchID)

		batchHeaders, err := neighbor.GetBlockHeaders(batchStartHeight, batchEndHeight)
		if err != nil {
			log.Warningf("Get block headers error: %v", err)
			return nil, false
		}

		return batchHeaders, true
	}

	saveHeader := func(batchID uint32, result interface{}) bool {
		batchHeaders, ok := result.([]*types.Header)
		if !ok {
			log.Warningf("Convert batch headers error")
			return false
		}

		batchStartHeight, batchEndHeight := getBatchHeightRange(batchID)
		for height := batchEndHeight; height >= batchStartHeight; height-- {
			header := batchHeaders[height-batchStartHeight]
			headerHash := header.Hash()
			if height == stopHeight && headerHash != stopHash {
				log.Warningf("End header hash %s is different from stop hash %s", (&headerHash).ToHexString(), stopHash.ToHexString())
				return false
			}
			if height < stopHeight {
				nextPrevHash, _ := common.Uint256ParseFromBytes(nextHeader.UnsignedHeader.PrevBlockHash)
				if nextHeader == nil || headerHash != nextPrevHash {
					log.Warningf("Header hash %s is different from prev hash in next block %s", (&headerHash).ToHexString(), nextHeader.UnsignedHeader.PrevBlockHash)
					return false
				}
			}

			prevHash, _ := common.Uint256ParseFromBytes(header.UnsignedHeader.PrevBlockHash)
			if height == startHeight && prevHash != startPrevHash {
				log.Warningf("Start header prev hash %s is different from start prev hash %s", header.UnsignedHeader.PrevBlockHash, startPrevHash.ToHexString())
				return false
			}
			headersHash[height-startHeight] = headerHash
			nextHeader = header
		}
		return true
	}

	cs, err := consequential.NewConSequential(&consequential.Config{
		StartJobID:          numBatches - 1,
		EndJobID:            0,
		JobBufSize:          config.Parameters.SyncBatchWindowSize,
		WorkerPoolSize:      numWorkers,
		MaxWorkerFails:      maxSyncWorkerFails,
		WorkerStartInterval: syncWorkerStartInterval,
		RunJob:              getHeader,
		FinishJob:           saveHeader,
	})
	if err != nil {
		return nil, err
	}

	err = cs.Start()
	if err != nil {
		return nil, err
	}

	return headersHash, nil
}

func (localNode *LocalNode) syncBlocks(startHeight, stopHeight uint32, neighbors []*RemoteNode, headersHash []common.Uint256) error {
	numBatches := (stopHeight-startHeight)/config.Parameters.SyncBlocksBatchSize + 1
	numWorkers := uint32(len(neighbors)) * concurrentSyncRequestPerNeighbor

	getBatchHeightRange := func(batchID uint32) (uint32, uint32) {
		batchStartHeight := startHeight + batchID*config.Parameters.SyncBlocksBatchSize
		batchEndHeight := batchStartHeight + config.Parameters.SyncBlocksBatchSize - 1
		if batchEndHeight > stopHeight {
			batchEndHeight = stopHeight
		}
		return batchStartHeight, batchEndHeight
	}

	getBlock := func(workerID, batchID uint32) (interface{}, bool) {
		neighbor := neighbors[workerID%uint32(len(neighbors))]
		batchStartHeight, batchEndHeight := getBatchHeightRange(batchID)

		batchBlocks, err := neighbor.GetBlocks(batchStartHeight, batchEndHeight)
		if err != nil {
			log.Warningf("Get blocks error: %v", err)
			return nil, false
		}

		return batchBlocks, true
	}

	saveBlock := func(batchID uint32, result interface{}) bool {
		batchBlocks, ok := result.([]*types.Block)
		if !ok {
			log.Warningf("Convert batch blocks error")
			return false
		}

		batchStartHeight, batchEndHeight := getBatchHeightRange(batchID)
		for height := batchStartHeight; height <= batchEndHeight; height++ {
			block := batchBlocks[height-batchStartHeight]
			blockHash := block.Header.Hash()
			headerHash := headersHash[height-startHeight]
			if blockHash != headerHash {
				log.Warningf("Block hash %s is different from header hash %s", (&blockHash).ToHexString(), (&headerHash).ToHexString())
				return false
			}

			err := core.DefaultLedger.Blockchain.AddBlock(block, false)
			if err != nil {
				return false
			}
		}

		return true
	}

	cs, err := consequential.NewConSequential(&consequential.Config{
		StartJobID:          0,
		EndJobID:            numBatches - 1,
		JobBufSize:          config.Parameters.SyncBatchWindowSize,
		WorkerPoolSize:      numWorkers,
		MaxWorkerFails:      maxSyncWorkerFails,
		WorkerStartInterval: syncWorkerStartInterval,
		RunJob:              getBlock,
		FinishJob:           saveBlock,
	})
	if err != nil {
		return err
	}

	err = cs.Start()
	if err != nil {
		return err
	}

	return nil
}
