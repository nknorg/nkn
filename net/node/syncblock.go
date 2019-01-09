package node

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/net/node/consequential"
	"github.com/nknorg/nkn/pb"
)

const (
	concurrentSyncRequestPerNeighbor = 1
	syncBatchWindowSize              = 1024
	syncBlockHeadersBatchSize        = 512
	maxSyncBlockHeadersBatchSize     = 4096
	syncBlocksBatchSize              = 32
	maxSyncBlocksBatchSize           = 256
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
func NewGetBlockHeadersReply(headers []*ledger.Header) (*pb.UnsignedMessage, error) {
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
func NewGetBlocksReply(blocks []*ledger.Block) (*pb.UnsignedMessage, error) {
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

	if endHeight > ledger.DefaultLedger.Store.GetHeaderHeight() {
		return replyBuf, false, nil
	}

	headers := make([]*ledger.Header, endHeight-startHeight+1, endHeight-startHeight+1)
	for height := startHeight; height <= endHeight; height++ {
		headers[height-startHeight], err = ledger.DefaultLedger.Store.GetHeaderByHeight(height)
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

	if endHeight > ledger.DefaultLedger.Store.GetHeight() {
		return replyBuf, false, nil
	}

	blocks := make([]*ledger.Block, endHeight-startHeight+1, endHeight-startHeight+1)
	for height := startHeight; height <= endHeight; height++ {
		blocks[height-startHeight], err = ledger.DefaultLedger.Store.GetBlockByHeight(height)
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
func (remoteNode *RemoteNode) GetBlockHeaders(startHeight, endHeight uint32) ([]*ledger.Header, error) {
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

	replyBytes, err := remoteNode.SendBytesSync(buf)
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

	headers := make([]*ledger.Header, len(replyMsg.BlockHeaders), len(replyMsg.BlockHeaders))
	for i := range replyMsg.BlockHeaders {
		headers[i] = &ledger.Header{}
		err = headers[i].Deserialize(bytes.NewReader(replyMsg.BlockHeaders[i]))
		if err != nil {
			return nil, err
		}
	}

	return headers, nil
}

// GetBlocks requests a range of consecutive blocks from a neighbor using
// GET_BLOCKS message
func (remoteNode *RemoteNode) GetBlocks(startHeight, endHeight uint32) ([]*ledger.Block, error) {
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

	replyBytes, err := remoteNode.SendBytesSync(buf)
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

	blocks := make([]*ledger.Block, len(replyMsg.Blocks), len(replyMsg.Blocks))
	for i := range replyMsg.Blocks {
		blocks[i] = &ledger.Block{}
		err = blocks[i].Deserialize(bytes.NewReader(replyMsg.Blocks[i]))
		if err != nil {
			return nil, err
		}
	}

	return blocks, nil
}

func (localNode *LocalNode) initSyncing() {
	localNode.AddMessageHandler(pb.GET_BLOCK_HEADERS, localNode.getBlockHeadersMessageHandler)
	localNode.AddMessageHandler(pb.GET_BLOCKS, localNode.getBlocksMessageHandler)
	localNode.ResetSyncing()
}

func (localNode *LocalNode) StartSyncing(stopHash common.Uint256, stopHeight uint32, neighbors []*RemoteNode) (bool, error) {
	localNode.Lock()
	defer localNode.Unlock()

	started := false
	var err error

	localNode.syncOnce.Do(func() {
		started = true
		localNode.SetSyncState(pb.SyncStarted)

		currentHeight := ledger.DefaultLedger.Store.GetHeight()
		currentHash := ledger.DefaultLedger.Store.GetHeaderHashByHeight(currentHeight)
		if stopHeight <= currentHeight {
			err = fmt.Errorf("sync stop height %d is not higher than current height %d", stopHeight, currentHeight)
			return
		}

		if len(neighbors) == 0 {
			err = fmt.Errorf("no neighbors to sync from")
			return
		}

		var headers []*ledger.Header
		headers, err = localNode.syncBlockHeaders(currentHeight+1, stopHeight, currentHash, stopHash, neighbors)
		if err != nil {
			return
		}

		err = localNode.syncBlocks(currentHeight+1, stopHeight, headers, neighbors)
		if err != nil {
			return
		}

		localNode.SetSyncState(pb.SyncFinished)
	})

	if err != nil {
		localNode.syncOnce = new(sync.Once)
	}

	return started, err
}

func (localNode *LocalNode) ResetSyncing() {
	localNode.Lock()
	defer localNode.Unlock()
	localNode.syncOnce = new(sync.Once)
}

func (localNode *LocalNode) syncBlockHeaders(startHeight, stopHeight uint32, startPrevHash, stopHash common.Uint256, neighbors []*RemoteNode) ([]*ledger.Header, error) {
	headers := make([]*ledger.Header, stopHeight-startHeight+1, stopHeight-startHeight+1)
	numBatches := (stopHeight-startHeight)/syncBlockHeadersBatchSize + 1
	numWorkers := uint32(len(neighbors)) * concurrentSyncRequestPerNeighbor

	getBatchHeightRange := func(batchID uint32) (uint32, uint32) {
		batchStartHeight := startHeight + batchID*syncBlockHeadersBatchSize
		batchEndHeight := batchStartHeight + syncBlockHeadersBatchSize - 1
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
			return nil, false
		}

		return batchHeaders, true
	}

	saveHeader := func(batchID uint32, result interface{}) bool {
		batchHeaders, ok := result.([]*ledger.Header)
		if !ok {
			return false
		}

		batchStartHeight, batchEndHeight := getBatchHeightRange(batchID)
		for height := batchEndHeight; height >= batchStartHeight; height-- {
			header := batchHeaders[height-batchStartHeight]
			if height == stopHeight && header.Hash() != stopHash {
				return false
			}
			if height < stopHeight {
				nextHeader := headers[height-startHeight+1]
				if nextHeader == nil || nextHeader.PrevBlockHash != header.Hash() {
					return false
				}
			}
			if height == startHeight && header.PrevBlockHash != startPrevHash {
				return false
			}
			headers[height-startHeight] = header
		}
		return true
	}

	cs, err := consequential.NewConSequential(numBatches-1, 0, syncBatchWindowSize, numWorkers, getHeader, saveHeader)
	if err != nil {
		return nil, err
	}

	cs.Start()

	err = ledger.DefaultLedger.Store.AddHeaders(headers)
	if err != nil {
		return nil, err
	}

	return headers, nil
}

func (localNode *LocalNode) syncBlocks(startHeight, stopHeight uint32, headers []*ledger.Header, neighbors []*RemoteNode) error {
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

	getBlock := func(workerID, batchID uint32) (interface{}, bool) {
		neighbor := neighbors[workerID%uint32(len(neighbors))]
		batchStartHeight, batchEndHeight := getBatchHeightRange(batchID)

		batchBlocks, err := neighbor.GetBlocks(batchStartHeight, batchEndHeight)
		if err != nil {
			return nil, false
		}

		return batchBlocks, true
	}

	saveBlock := func(batchID uint32, result interface{}) bool {
		batchBlocks, ok := result.([]*ledger.Block)
		if !ok {
			return false
		}

		batchStartHeight, batchEndHeight := getBatchHeightRange(batchID)
		for height := batchStartHeight; height <= batchEndHeight; height++ {
			block := batchBlocks[height-batchStartHeight]
			if block.Header.Hash() != headers[height-startHeight].Hash() {
				return false
			}
			err := ledger.DefaultLedger.Blockchain.AddBlock(block)
			if err != nil {
				return false
			}
		}
		return true
	}

	cs, err := consequential.NewConSequential(0, numBatches-1, syncBatchWindowSize, numWorkers, getBlock, saveBlock)
	if err != nil {
		return err
	}

	cs.Start()

	return nil
}
