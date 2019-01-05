package node

import (
	"math/rand"
	"sync"
	"time"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/timer"
)

const (
	syncBlockInterval = 500 * time.Millisecond
)

func (node *Node) startSyncingBlock() {
	syncBlockTimer := time.NewTimer(0)
	for {
		select {
		case <-syncBlockTimer.C:
			if node.syncStopHash == common.EmptyUint256 {
				break
			}

			syncState := node.GetSyncState()
			if syncState != pb.WaitForSyncing && syncState != pb.PersistFinished {
				break
			}

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				node.SyncBlock(false)
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				node.SyncBlockMonitor(false)
			}()

			wg.Wait()
		}
		timer.ResetTimer(syncBlockTimer, syncBlockInterval)
	}
}

func (node *Node) WaitForSyncHeaderFinish(isProposer bool) {
	if isProposer {
		for {
			//TODO: proposer node syncs block from 50% neighbors
			heights, _ := node.GetNeighborHeights()
			if common.CompareHeight(ledger.DefaultLedger.Blockchain.BlockHeight, heights) {
				break
			}
			<-time.After(time.Second)
		}
	} else {
		for {
			if node.syncStopHash != common.EmptyUint256 {
				// return if the stop hash has been saved
				header, err := ledger.DefaultLedger.Blockchain.GetHeader(node.syncStopHash)
				if err == nil && header != nil {
					break
				}
			}
			<-time.After(time.Second)
		}
	}
}

func (node *Node) WaitForSyncBlkFinish() {
	for {
		headerHeight := ledger.DefaultLedger.Store.GetHeaderHeight()
		currentBlkHeight := ledger.DefaultLedger.Blockchain.BlockHeight
		log.Debug("WaitForSyncBlkFinish... current block height is ", currentBlkHeight, " ,current header height is ", headerHeight)
		if currentBlkHeight >= headerHeight {
			break
		}
		<-time.After(2 * time.Second)
	}
}

func (node *Node) StoreFlightHeight(height uint32) {
	node.flightHeights = append(node.flightHeights, height)
}

func (node *Node) GetFlightHeightCnt() int {
	return len(node.flightHeights)
}

func (node *Node) GetFlightHeights() []uint32 {
	return node.flightHeights
}

func (node *Node) RemoveFlightHeightLessThan(h uint32) {
	heights := node.flightHeights
	p := len(heights)
	i := 0

	for i < p {
		if heights[i] < h {
			p--
			heights[p], heights[i] = heights[i], heights[p]
		} else {
			i++
		}
	}
	node.flightHeights = heights[:p]
}

func (node *Node) RemoveFlightHeight(height uint32) {
	node.flightHeights = common.SliceRemove(node.flightHeights, height)
}

func (node *Node) blockHeaderSyncing(stopHash common.Uint256) {
	noders := node.local.GetNeighborNoder(nil)
	if len(noders) == 0 {
		return
	}
	nodelist := []protocol.Noder{}
	for _, v := range noders {
		if ledger.DefaultLedger.Store.GetHeaderHeight() < v.GetHeight() {
			nodelist = append(nodelist, v)
		}
	}
	ncout := len(nodelist)
	if ncout == 0 {
		return
	}
	index := rand.Intn(ncout)
	n := nodelist[index]
	message.SendMsgSyncHeaders(n, stopHash)
}

func (node *Node) blockSyncing() {
	headerHeight := ledger.DefaultLedger.Store.GetHeaderHeight()
	currentBlkHeight := ledger.DefaultLedger.Blockchain.BlockHeight
	if currentBlkHeight >= headerHeight {
		return
	}
	var dValue int32
	var reqCnt uint32
	var i uint32
	noders := node.local.GetNeighborNoder(nil)

	for _, n := range noders {
		if uint32(n.GetHeight()) <= currentBlkHeight {
			continue
		}
		n.RemoveFlightHeightLessThan(currentBlkHeight)
		count := MaxReqBlkOnce - uint32(n.GetFlightHeightCnt())
		dValue = int32(headerHeight - currentBlkHeight - reqCnt)
		flights := n.GetFlightHeights()
		if count == 0 {
			for _, f := range flights {
				hash := ledger.DefaultLedger.Store.GetHeaderHashByHeight(f)
				if !ledger.DefaultLedger.Store.BlockInCache(hash) {
					message.ReqBlkData(n, hash)
				}
			}

		}
		for i = 1; i <= count && dValue >= 0; i++ {
			hash := ledger.DefaultLedger.Store.GetHeaderHashByHeight(currentBlkHeight + reqCnt)

			if !ledger.DefaultLedger.Store.BlockInCache(hash) {
				message.ReqBlkData(n, hash)
				n.StoreFlightHeight(currentBlkHeight + reqCnt)
			}
			reqCnt++
			dValue--
		}
	}
}

func (node *Node) SyncBlock(isProposer bool) {
	log.Infof("Start syncing block")
	node.SetSyncState(pb.SyncStarted)
	ticker := time.NewTicker(BlockSyncingTicker)
	for {
		select {
		case <-ticker.C:
			if isProposer {
				node.blockHeaderSyncing(common.EmptyUint256)
			} else if node.syncStopHash != common.EmptyUint256 {
				node.blockHeaderSyncing(node.syncStopHash)
			}
			node.blockSyncing()
		case skip := <-node.quit:
			log.Info("block syncing finished")
			ticker.Stop()
			node.LocalNode().GetEvent("sync").Notify(events.EventBlockSyncingFinished, skip)
			return
		}
	}
}

func (node *Node) StopSyncBlock(skip bool) {
	log.Infof("Sync block Finished")
	node.SetSyncState(pb.SyncFinished)
	node.quit <- skip
}

func (node *Node) SyncBlockMonitor(isProposer bool) {
	// wait for header syncing finished
	node.WaitForSyncHeaderFinish(isProposer)
	// wait for block syncing finished
	node.WaitForSyncBlkFinish()
	// stop block syncing
	node.StopSyncBlock(false)
}
