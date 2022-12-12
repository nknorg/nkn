package lnode

import (
	"bytes"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/chain/pool"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/node"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/transaction"
	"github.com/nknorg/nkn/v2/util"
	"github.com/nknorg/nkn/v2/util/log"
)

// Number of random neighbors to sync txn pool
const NumNbrToSyncTxnPool = 1

func (localNode *LocalNode) StartSyncTxnPool() {
	for {
		dur := util.RandDuration(config.ConsensusDuration/2, 0.4)
		time.Sleep(dur)
		localNode.SyncTxnPool()
	}
}

// sync txn pool process
func (localNode *LocalNode) SyncTxnPool() {

	// remove expired twt before sending request
	localNode.TxnPool.RemoveExpiredTwt()

	neighbors := localNode.GetNeighbors(nil)
	neighborCount := len(neighbors)

	if neighborCount <= 0 { // don't have any neighbor yet
		return
	}

	if NumNbrToSyncTxnPool < neighborCount { // choose random neighbors

		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		r.Shuffle(len(neighbors), func(i, j int) { neighbors[i], neighbors[j] = neighbors[j], neighbors[i] })

		neighbors = neighbors[:NumNbrToSyncTxnPool]
	}

	myHash, _ := localNode.TxnPool.GetTwtFingerprintAndCount()
	maxCount := uint32(0)
	var maxMu sync.Mutex
	var neighborToSync *node.RemoteNode // the neighbor which is sync txn target
	var wg sync.WaitGroup

	for _, neighbor := range neighbors {
		wg.Add(1)

		go func(nbr *node.RemoteNode) {
			defer wg.Done()

			replyMsg, err := localNode.requestTxnPoolHash(nbr)
			if err != nil {
				log.Warningf("sendReqTxnPoolHash neighbor %v error: %v", nbr.GetAddr(), err)
				return
			}

			if !bytes.Equal(replyMsg.PoolHash, myHash) {
				maxMu.Lock()
				if replyMsg.TxnCount > maxCount { // Choose the neighbor which has most txn count in txn pool
					neighborToSync = nbr
					maxCount = replyMsg.TxnCount
				}
				maxMu.Unlock()
			}
		}(neighbor)

	}

	wg.Wait()

	// request for syncing
	if neighborToSync != nil {
		replyMsg, err := localNode.requestSyncTxnPool(neighborToSync)
		if err != nil {
			log.Error("req sync txn pool error: ", err)
		} else {
			// sort it by nonce to append them to txn pool.
			sort.Slice(replyMsg.Transactions, func(i, j int) bool {
				return replyMsg.Transactions[i].UnsignedTx.Nonce < replyMsg.Transactions[j].UnsignedTx.Nonce
			})

			for _, pbTxn := range replyMsg.Transactions {
				txn := &transaction.Transaction{
					Transaction: pbTxn,
				}

				if localNode.ExistHash(txn.Hash()) {
					continue
				}
				localNode.TxnPool.AppendTxnPool(txn)
			}

			// after update my txn pool, reset sync time.
			localNode.TxnPool.SetSyncTime()
		}
	}
}

// send neighbor a REQ_TXN_POOL_HASH request message, and wait for reply.
func (localNode *LocalNode) requestTxnPoolHash(remoteNode *node.RemoteNode) (*pb.TxnPoolHashAndCount, error) {

	msg, err := NewReqTxnPoolHashMessage()
	if err != nil {
		return nil, err
	}

	buf, err := localNode.SerializeMessage(msg, false)
	if err != nil {
		return nil, err
	}

	replyBytes, err := remoteNode.SendBytesSyncWithTimeout(buf, syncReplyTimeout)
	if err != nil {
		return nil, err
	}

	replyMsg := &pb.TxnPoolHashAndCount{}
	err = proto.Unmarshal(replyBytes, replyMsg)
	if err != nil {
		return nil, err
	}

	return replyMsg, nil
}

// build a REQ_TXN_POOL_HASH request message
func NewReqTxnPoolHashMessage() (*pb.UnsignedMessage, error) {

	msg := &pb.UnsignedMessage{
		MessageType: pb.MessageType_REQ_TXN_POOL_HASH,
	}

	return msg, nil

}

// build RPL_TXN_POOL_HASH reply message, reply with my txn pool's hash and txn count.
func NewRplTxnPoolHashMessage(tp *pool.TxnPool) (*pb.UnsignedMessage, error) {

	poolHash, txnCount := tp.GetTwtFingerprintAndCount()

	msgBody := &pb.TxnPoolHashAndCount{
		PoolHash: poolHash,
		TxnCount: (uint32)(txnCount),
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.MessageType_RPL_TXN_POOL_HASH,
		Message:     buf,
	}

	return msg, nil
}

// build REQ_SYNC_TXN_POOL request message. This message includse my txn pool's txn address and maximum nonce.
func NewReqSyncTxnPoolMessage(tp *pool.TxnPool) (*pb.UnsignedMessage, error) {

	addrNonceList := make([]*pb.AddrNonce, 0)
	mapAddrNonce := tp.GetTwtAddrNonce()

	for addr, nonce := range mapAddrNonce {
		addrNonceList = append(addrNonceList, &pb.AddrNonce{
			Address: addr.ToArray(),
			Nonce:   nonce,
		})
	}

	// get the duration since last sync txn time.
	duration := time.Now().UnixMilli() - tp.GetSyncTime()
	if tp.GetSyncTime() == 0 { // never synced before
		duration = 0
	}
	msgBody := &pb.RequestSyncTxnPool{Duration: duration, AddrNonce: addrNonceList}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.MessageType_REQ_SYNC_TXN_POOL,
		Message:     buf,
	}

	return msg, nil
}

// build RPL_SYNC_TXN_POOL message. reply transactions which are needed to sync to the requestor.
func NewRplSyncTxnPoolMessage(reqMsg *pb.RequestSyncTxnPool, tp *pool.TxnPool) (*pb.UnsignedMessage, error) {

	mapReqAddrNonce := make(map[common.Uint160]uint64)

	// if there is address and nonce in the request message, save them in a map for better searching.
	if len(reqMsg.AddrNonce) > 0 {
		for _, addrNonce := range reqMsg.AddrNonce {
			mapReqAddrNonce[common.BytesToUint160(addrNonce.Address)] = addrNonce.Nonce
		}
	}

	var respTxns []*pb.Transaction
	var err error

	if reqMsg.Duration > 0 {
		respTxns, err = syncInDuration(tp, mapReqAddrNonce, reqMsg.Duration)
	} else {
		respTxns, err = syncFully(tp, mapReqAddrNonce)
	}
	if err != nil {
		return nil, err
	}

	msgBody := &pb.Transactions{Transactions: respTxns}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.MessageType_RPL_SYNC_TXN_POOL,
		Message:     buf,
	}

	return msg, nil
}

// Only sync txn in the last duration seconds, partially of the txn pool
func syncInDuration(tp *pool.TxnPool, reqAddrNonce map[common.Uint160]uint64, duration int64) ([]*pb.Transaction, error) {

	now := time.Now().UnixMilli()
	earliest := now - duration - 1000 // push 1 second ahead
	respTxns, err := tp.GetTwtTxnAfter(earliest, reqAddrNonce)

	return respTxns, err
}

// sync all my txns in the pool to the neighbor when the neighbor just start.
func syncFully(tp *pool.TxnPool, reqAddrNonce map[common.Uint160]uint64) ([]*pb.Transaction, error) {

	respTxns := make([]*pb.Transaction, 0) // response txn list buffer

	mapMyTxnList := tp.GetAllTransactionLists()
	for addr, myTxnList := range mapMyTxnList {
		nonce, _ := reqAddrNonce[addr]
		for _, txn := range myTxnList {
			if txn.UnsignedTx.Nonce >= nonce {
				respTxns = append(respTxns, txn.Transaction)
			}
		}
	}

	log.Info("Sync txn, send ", len(respTxns), " txns to neighbor")

	return respTxns, nil
}

// handle a REQ_TXN_POOL_HASH request message, return a RPL_TXN_POOL_HASH response message buffer.
func (localNode *LocalNode) requestTxnPoolHashHandler(remoteMessage *node.RemoteMessage) ([]byte, bool, error) {
	// remove expired twt when getting request
	localNode.TxnPool.RemoveExpiredTwt()

	replyMsg, err := NewRplTxnPoolHashMessage(localNode.TxnPool)
	if err != nil {
		return nil, false, err
	}

	replyBuf, err := localNode.SerializeMessage(replyMsg, false)
	return replyBuf, false, err
}

// send to a neighbor a REQ_SYNC_TXN_POOL request message, and wait for reply with timeout
// return the reply messge RPL_SYNC_TXN_POOL
func (localNode *LocalNode) requestSyncTxnPool(remoteNode *node.RemoteNode) (*pb.Transactions, error) {

	msg, err := NewReqSyncTxnPoolMessage(localNode.TxnPool)
	if err != nil {
		return nil, err
	}

	buf, err := localNode.SerializeMessage(msg, false)
	if err != nil {
		return nil, err
	}

	replyBytes, err := remoteNode.SendBytesSyncWithTimeout(buf, syncReplyTimeout)
	if err != nil {
		return nil, err
	}

	// get RSP_SYNC_TXN_POOL message, update my txn pool
	replyMsg := &pb.Transactions{}
	err = proto.Unmarshal(replyBytes, replyMsg)
	if err != nil {
		return nil, err
	}

	log.Info("Sync txn pool, get ", len(replyMsg.Transactions), " txns from neighbor")

	return replyMsg, nil
}

// Handle REQ_SYNC_TXN_POOL request message, return RPL_SYNC_TXN_POOL response message buffer.
func (localNode *LocalNode) requestSyncTxnPoolHandler(remoteMessage *node.RemoteMessage) ([]byte, bool, error) {

	reqMsg := &pb.RequestSyncTxnPool{}
	err := proto.Unmarshal(remoteMessage.Message, reqMsg)
	if err != nil {
		return nil, false, err
	}

	replyMsg, err := NewRplSyncTxnPoolMessage(reqMsg, localNode.TxnPool)
	if err != nil {
		return nil, false, err
	}

	replyBuf, err := localNode.SerializeMessage(replyMsg, false)
	return replyBuf, false, err
}
