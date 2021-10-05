package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/consequential"
	"github.com/nknorg/nkn/v2/block"
	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/util/log"
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
		MessageType: pb.MessageType_GET_BLOCK_HEADERS,
		Message:     buf,
	}

	return msg, nil
}

// NewGetBlockHeadersReply creates a GET_BLOCK_HEADERS_REPLY message in respond
// to GET_BLOCK_HEADERS message
func NewGetBlockHeadersReply(headers []*block.Header) (*pb.UnsignedMessage, error) {
	pbHeaders := make([]*pb.Header, len(headers))
	for i, header := range headers {
		pbHeaders[i] = header.Header
	}

	msgBody := &pb.GetBlockHeadersReply{
		BlockHeaders: pbHeaders,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.MessageType_GET_BLOCK_HEADERS_REPLY,
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
		MessageType: pb.MessageType_GET_BLOCKS,
		Message:     buf,
	}

	return msg, nil
}

// NewGetBlocksReply creates a GET_BLOCKS_REPLY message in respond to GET_BLOCKS
// message
func NewGetBlocksReply(blocks []*block.Block) (*pb.UnsignedMessage, error) {
	msgBlocks := make([]*pb.Block, len(blocks))
	for i, b := range blocks {
		msgBlocks[i] = b.ToMsgBlock()
	}

	msgBody := &pb.GetBlocksReply{
		Blocks: msgBlocks,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.MessageType_GET_BLOCKS_REPLY,
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

	if !localNode.syncHeaderLimiter.AllowN(time.Now(), int(endHeight-startHeight)) {
		return replyBuf, false, nil
	}

	if endHeight > chain.DefaultLedger.Store.GetHeaderHeight() {
		return replyBuf, false, nil
	}

	headers := make([]*block.Header, endHeight-startHeight+1, endHeight-startHeight+1)
	for height := startHeight; height <= endHeight; height++ {
		headers[height-startHeight], err = chain.DefaultLedger.Store.GetHeaderByHeight(height)
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

	if !localNode.syncBlockLimiter.AllowN(time.Now(), int(endHeight-startHeight)) {
		return replyBuf, false, nil
	}

	if endHeight > chain.DefaultLedger.Store.GetHeight() {
		return replyBuf, false, nil
	}

	blocks := make([]*block.Block, endHeight-startHeight+1, endHeight-startHeight+1)
	for height := startHeight; height <= endHeight; height++ {
		blocks[height-startHeight], err = chain.DefaultLedger.Store.GetBlockByHeight(height)
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
func (remoteNode *RemoteNode) GetBlockHeaders(startHeight, endHeight uint32) ([]*block.Header, error) {
	if startHeight > endHeight {
		return nil, fmt.Errorf("start height %d is higher than end height %d", startHeight, endHeight)
	}

	msg, err := NewGetBlockHeadersMessage(startHeight, endHeight)
	if err != nil {
		return nil, err
	}

	buf, err := remoteNode.localNode.SerializeMessage(msg, false)
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

	headers := make([]*block.Header, len(replyMsg.BlockHeaders))
	for i, msgHeader := range replyMsg.BlockHeaders {
		headers[i] = &block.Header{Header: msgHeader}
	}

	return headers, nil
}

// GetBlocks requests a range of consecutive blocks from a neighbor using
// GET_BLOCKS message
func (remoteNode *RemoteNode) GetBlocks(startHeight, endHeight uint32) ([]*block.Block, error) {
	if startHeight > endHeight {
		return nil, fmt.Errorf("start height %d is higher than end height %d", startHeight, endHeight)
	}

	msg, err := NewGetBlocksMessage(startHeight, endHeight)
	if err != nil {
		return nil, err
	}

	buf, err := remoteNode.localNode.SerializeMessage(msg, false)
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

	blocks := make([]*block.Block, len(replyMsg.Blocks))
	for i, msgBlock := range replyMsg.Blocks {
		blocks[i] = &block.Block{}
		blocks[i].FromMsgBlock(msgBlock)
	}

	return blocks, nil
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
	header := localNode.getNeighborsMajorityBlockByHeight(height, neighbors)
	if header == nil {
		return common.EmptyUint256
	}
	return header.Hash()
}

// getNeighborsMajorityStateRootByHeight returns the majority of given
// neighbors' block hash at a give height
func (localNode *LocalNode) getNeighborsMajorityStateRootByHeight(height uint32, neighbors []*RemoteNode) common.Uint256 {
	header := localNode.getNeighborsMajorityBlockByHeight(height, neighbors)
	if header == nil {
		return common.EmptyUint256
	}
	rootHash, err := common.Uint256ParseFromBytes(header.UnsignedHeader.StateRoot)
	if err != nil {
		return common.EmptyUint256
	}
	return rootHash
}

// getNeighborsMajorityBlockByHeight returns the majority of given
// neighbors' block hash at a give height
func (localNode *LocalNode) getNeighborsMajorityBlockByHeight(height uint32, neighbors []*RemoteNode) *block.Header {
	for i := 0; i < 3; i++ {
		allHeaders, err := localNode.getNeighborsBlockHeaderByHeight(height, neighbors)
		if err != nil {
			log.Warningf("Get neighbors block header at height %d error: %v", height, err)
			continue
		}

		counter := make(map[common.Uint256]int)
		headers := make(map[common.Uint256]*block.Header)
		totalCount := 0
		allHeaders.Range(func(key, value interface{}) bool {
			if header, ok := value.(*block.Header); ok && header != nil {
				counter[header.Hash()]++
				if _, ok := headers[header.Hash()]; !ok {
					h := value.(*block.Header)
					if h.UnsignedHeader.Height > 0 {
						err = h.VerifySignature()
					}
					if err == nil {
						headers[header.Hash()] = h
					} else {
						log.Infof("Received header with invalid signature from neighbor %s", key)
					}
				}
				totalCount++
			}
			return true
		})

		if totalCount == 0 {
			continue
		}

		for blockHash, count := range counter {
			if count > int(rollbackMinRelativeWeight*float32(totalCount)) {
				return headers[blockHash]
			}
		}
	}

	return nil
}

func (localNode *LocalNode) GetNeighborsMajorityStateRootByHeight(height uint32, neighbors []*RemoteNode) common.Uint256 {
	return localNode.getNeighborsMajorityStateRootByHeight(height, neighbors)
}

// initSyncing initializes block syncing state and registers message handler
func (localNode *LocalNode) initSyncing() {
	localNode.AddMessageHandler(pb.MessageType_GET_BLOCK_HEADERS, localNode.getBlockHeadersMessageHandler)
	localNode.AddMessageHandler(pb.MessageType_GET_BLOCKS, localNode.getBlocksMessageHandler)
	localNode.AddMessageHandler(pb.MessageType_GET_STATES, localNode.getStatesMessageHandler)
	localNode.ResetSyncing()
}

func removeStoppedNeighbors(neighbors []*RemoteNode) []*RemoteNode {
	availableNeighbors := make([]*RemoteNode, 0, len(neighbors))
	for _, n := range neighbors {
		if !n.IsStopped() {
			availableNeighbors = append(availableNeighbors, n)
		}
	}
	return availableNeighbors
}

// StartSyncing starts block syncing from current local ledger until it gets to
// block height stopHeight with block hash stopHash from given neighbors
func (localNode *LocalNode) StartSyncing(syncStopHash common.Uint256, syncStopHeight uint32, neighbors []*RemoteNode) (bool, error) {
	var err error
	started := false

	localNode.mu.RLock()
	syncOnce := localNode.syncOnce
	localNode.mu.RUnlock()

	syncOnce.Do(func() {
		started = true
		localNode.SetSyncState(pb.SyncState_SYNC_STARTED)

		cs := chain.DefaultLedger.Store

		var fastSyncHeight uint32
		fastSyncHeight, err = cs.GetSyncRootHeight()
		if err != nil {
			log.Info("local sync root height not found:", err)
		}

		if !config.Parameters.IsFastSync() && fastSyncHeight > cs.GetHeight() {
			err = fmt.Errorf("invalid sync mode %s at height %d", config.Parameters.SyncMode, cs.GetHeight())
			return
		}

		if config.Parameters.IsFastSync() && cs.ShouldFastSync(syncStopHeight) {
			if fastSyncHeight == 0 {
				fastSyncHeight = syncStopHeight - config.Parameters.RecentStateCount
			}
			var fastSyncRootHash common.Uint256
			fastSyncRootHash, err = cs.GetFastSyncStateRoot()
			if err != nil {
				fastSyncRootHash = localNode.GetNeighborsMajorityStateRootByHeight(fastSyncHeight, removeStoppedNeighbors(neighbors))
				if fastSyncRootHash == common.EmptyUint256 {
					err = fmt.Errorf("get neighbors majority state root of height %d failed", fastSyncHeight)
					return
				}
			}

			err = cs.PrepareFastSync(fastSyncHeight, fastSyncRootHash)
			if err != nil {
				err = fmt.Errorf("prepare fast sync err: %v", err)
				return
			}
			err = localNode.StartFastSyncing(fastSyncRootHash, removeStoppedNeighbors(neighbors))
			if err != nil {
				err = fmt.Errorf("start fast syncing err: %v", err)
				return
			}
			err = cs.FastSyncDone(fastSyncRootHash, fastSyncHeight)
			if err != nil {
				err = fmt.Errorf("fast sync done err: %v", err)
				return
			}
			log.Info("fast sync done")
		}

		currentHeight := cs.GetHeight()
		if syncStopHeight <= currentHeight {
			err = fmt.Errorf("sync stop height %d is not higher than current height %d", syncStopHeight, currentHeight)
			return
		}

		_, err = localNode.maybeRollback(removeStoppedNeighbors(neighbors))
		if err != nil {
			err = fmt.Errorf("rollback error: %v", err)
			return
		}

		for startHeight := currentHeight + 1; startHeight <= syncStopHeight; startHeight += config.Parameters.SyncHeaderMaxSize {
			stopHeight := startHeight + config.Parameters.SyncHeaderMaxSize - 1
			if stopHeight > syncStopHeight {
				stopHeight = syncStopHeight
			}

			startPrevHash := cs.GetHeaderHashByHeight(startHeight - 1)
			if startPrevHash == common.EmptyUint256 {
				err = fmt.Errorf("get block hash of height %d failed", startHeight-1)
				return
			}

			stopHash := syncStopHash
			if stopHeight < syncStopHeight {
				stopHash = localNode.getNeighborsMajorityBlockHashByHeight(stopHeight, removeStoppedNeighbors(neighbors))
				if stopHash == common.EmptyUint256 {
					err = fmt.Errorf("get neighbors majority block hash of height %d failed", stopHeight)
					return
				}
			}

			startTime := time.Now()
			var headersHash []common.Uint256
			var headers []*block.Header
			needFullHeaders := config.Parameters.IsLightSync() && startHeight < fastSyncHeight
			headersHash, headers, err = localNode.syncBlockHeaders(startHeight, stopHeight, startPrevHash, stopHash, removeStoppedNeighbors(neighbors), needFullHeaders)
			if err != nil {
				err = fmt.Errorf("sync block headers error: %v", err)
				return
			}

			log.Infof("Synced %d block headers in %s", stopHeight-startHeight+1, time.Since(startTime))

			var fullBlockStartHeight uint32
			if config.Parameters.IsLightSync() && fastSyncHeight > config.Parameters.RecentBlockCount {
				fullBlockStartHeight = fastSyncHeight - config.Parameters.RecentBlockCount

				for height := startHeight; height <= stopHeight && height < fullBlockStartHeight; height++ {
					err = chain.DefaultLedger.Blockchain.AddHeader(headers[height-startHeight], true)
					if err != nil {
						return
					}
				}

				if fullBlockStartHeight > stopHeight {
					continue
				}
			}

			startTime = time.Now()
			for syncBlocksBatchSize := config.Parameters.SyncBlocksBatchSize; syncBlocksBatchSize > 0; syncBlocksBatchSize /= 2 {
				batchStartHeight := cs.GetHeight() + 1
				if batchStartHeight < fullBlockStartHeight {
					batchStartHeight = fullBlockStartHeight
				}
				err = localNode.syncBlocks(batchStartHeight, stopHeight, syncBlocksBatchSize, removeStoppedNeighbors(neighbors), headersHash[batchStartHeight-startHeight:], fastSyncHeight)
				if err == nil {
					log.Infof("Synced %d blocks in %s", stopHeight-batchStartHeight+1, time.Since(startTime))
					break
				}
				log.Errorf("Sync blocks error with batch size %d: %v", syncBlocksBatchSize, err)
			}

			if err != nil {
				err = fmt.Errorf("sync blocks error: %v", err)
				return
			}
		}

		localNode.SetSyncState(pb.SyncState_SYNC_FINISHED)
	})

	return started, err
}

// ResetSyncing resets syncOnce and allows for future block syncing
func (localNode *LocalNode) ResetSyncing() {
	localNode.mu.Lock()
	defer localNode.mu.Unlock()
	localNode.syncOnce = new(sync.Once)
}

func (localNode *LocalNode) syncBlockHeaders(startHeight, stopHeight uint32, startPrevHash, stopHash common.Uint256, neighbors []*RemoteNode, fullHeader bool) ([]common.Uint256, []*block.Header, error) {
	var nextHeader *block.Header
	headersHash := make([]common.Uint256, stopHeight-startHeight+1, stopHeight-startHeight+1)
	var headers []*block.Header
	if fullHeader {
		headers = make([]*block.Header, stopHeight-startHeight+1, stopHeight-startHeight+1)
	}
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

	getHeader := func(ctx context.Context, workerID, batchID uint32) (interface{}, bool) {
		neighbor := neighbors[workerID%uint32(len(neighbors))]
		batchStartHeight, batchEndHeight := getBatchHeightRange(batchID)

		batchHeaders, err := neighbor.GetBlockHeaders(batchStartHeight, batchEndHeight)
		if err != nil {
			log.Warningf("Get block headers error: %v", err)
			return nil, false
		}

		return batchHeaders, true
	}

	saveHeader := func(ctx context.Context, batchID uint32, result interface{}) bool {
		batchHeaders, ok := result.([]*block.Header)
		if !ok {
			log.Warningf("Convert batch headers error")
			return false
		}

		batchStartHeight, batchEndHeight := getBatchHeightRange(batchID)
		for height := batchEndHeight; height >= batchStartHeight; height-- {
			header := batchHeaders[height-batchStartHeight]
			headerHash := header.Hash()
			if height == stopHeight && headerHash != stopHash {
				log.Warningf("End header hash %s is different from stop hash %s", headerHash.ToHexString(), stopHash.ToHexString())
				return false
			}
			if height < stopHeight {
				nextPrevHash, _ := common.Uint256ParseFromBytes(nextHeader.UnsignedHeader.PrevBlockHash)
				if nextHeader == nil || headerHash != nextPrevHash {
					log.Warningf("Header hash %s is different from prev hash in next block %s", headerHash.ToHexString(), nextPrevHash.ToHexString())
					return false
				}
			}

			prevHash, _ := common.Uint256ParseFromBytes(header.UnsignedHeader.PrevBlockHash)
			if height == startHeight && prevHash != startPrevHash {
				log.Warningf("Start header prev hash %s is different from start prev hash %s", prevHash.ToHexString(), startPrevHash.ToHexString())
				return false
			}
			headersHash[height-startHeight] = headerHash
			if fullHeader {
				headers[height-startHeight] = header
			}
			nextHeader = header
		}
		return true
	}

	cs, err := consequential.NewConSequential(&consequential.Config{
		StartJobID:          numBatches - 1,
		EndJobID:            0,
		JobBufSize:          numBatches,
		WorkerPoolSize:      numWorkers,
		MaxWorkerFails:      maxSyncWorkerFails,
		WorkerStartInterval: syncWorkerStartInterval,
		RunJob:              getHeader,
		FinishJob:           saveHeader,
	})
	if err != nil {
		return nil, nil, err
	}

	err = cs.Start(context.Background())
	if err != nil {
		return nil, nil, err
	}

	return headersHash, headers, nil
}

func (localNode *LocalNode) syncBlocks(startHeight, stopHeight, syncBlocksBatchSize uint32, neighbors []*RemoteNode, headersHash []common.Uint256, fastSyncHeight uint32) error {
	numBatches := (stopHeight-startHeight)/syncBlocksBatchSize + 1
	numWorkers := uint32(len(neighbors)) * concurrentSyncRequestPerNeighbor

	getBatchHeightRange := func(batchID uint32) (uint32, uint32) {
		batchStartHeight := startHeight + batchID*syncBlocksBatchSize
		batchEndHeight := batchStartHeight + syncBlocksBatchSize - 1
		if batchEndHeight > stopHeight {
			batchEndHeight = stopHeight
		}
		return batchStartHeight, batchEndHeight
	}

	getBlock := func(ctx context.Context, workerID, batchID uint32) (interface{}, bool) {
		neighbor := neighbors[workerID%uint32(len(neighbors))]
		batchStartHeight, batchEndHeight := getBatchHeightRange(batchID)

		batchBlocks, err := neighbor.GetBlocks(batchStartHeight, batchEndHeight)
		if err != nil {
			log.Warningf("Get blocks error: %v", err)
			return nil, false
		}

		return batchBlocks, true
	}

	saveBlock := func(ctx context.Context, batchID uint32, result interface{}) bool {
		batchBlocks, ok := result.([]*block.Block)
		if !ok {
			log.Warningf("Convert batch blocks error")
			return false
		}

		batchStartHeight, batchEndHeight := getBatchHeightRange(batchID)
		for height := batchStartHeight; height <= batchEndHeight; height++ {
			fastSync := false
			b := batchBlocks[height-batchStartHeight]
			blockHash := b.Hash()
			headerHash := headersHash[height-startHeight]
			if blockHash != headerHash {
				log.Warningf("Block hash %s is different from header hash %s", (&blockHash).ToHexString(), (&headerHash).ToHexString())
				return false
			}
			if height <= fastSyncHeight {
				fastSync = true
			}

			err := chain.DefaultLedger.Blockchain.AddBlock(b, config.SyncPruning, fastSync)
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

	err = cs.Start(context.Background())
	if err != nil {
		return err
	}

	return nil
}

// StartFastSyncing quickly download the headers, full sync only at the chain
func (localNode *LocalNode) StartFastSyncing(syncRootHash common.Uint256, peers []*RemoteNode) error {
	db := chain.DefaultLedger.Store.GetDatabase()
	s := newStateSync(db, syncRootHash, peers)
	return s.run()
}
