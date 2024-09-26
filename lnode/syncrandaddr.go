package lnode

import (
	"math/rand"
	"reflect"
	"time"

	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/chain/pool"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/node"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/transaction"
	"github.com/nknorg/nkn/v2/util"
	"github.com/nknorg/nkn/v2/util/log"
	"google.golang.org/protobuf/proto"
)

// Number of random neighbors to sync rand addr
const NumNbrToSyncRandAddr = 1

func (localNode *LocalNode) StartSyncRandAddrTxn() {
	for {
		dur := util.RandDuration(config.ConsensusDuration/2, 0.4)
		time.Sleep(dur)
		localNode.SyncRandAddrTxn()
	}
}

// sync rand addr process
func (localNode *LocalNode) SyncRandAddrTxn() {

	neighbors := localNode.GetNeighbors(nil)
	neighborCount := len(neighbors)
	if neighborCount <= 0 { // don't have neighbor yet
		return
	}

	if NumNbrToSyncRandAddr < neighborCount {
		// choose rand neighbors
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		r.Shuffle(len(neighbors), func(i, j int) { neighbors[i], neighbors[j] = neighbors[j], neighbors[i] })

		neighbors = neighbors[:NumNbrToSyncRandAddr]
	}

	for _, neighbor := range neighbors {

		go func(nbr *node.RemoteNode) {

			addrNonceReply, err := localNode.requestAddrNonce(nbr)
			if err != nil {
				log.Warningf("requestAddrNonce to neighbor %v error: %v\n", nbr.GetAddr(), err)
				return
			}

			addr160, err := common.Uint160ParseFromBytes(addrNonceReply.Address)
			if err != nil {
				log.Warning("Sync txn by rand address, parse replied address error : ", err.Error())
				return
			}

			if addr160.CompareTo(common.EmptyUint160) == 0 { // get an empty address
				return
			}

			// here we get next expected nonce = last nonce + 1
			myNonce, err := localNode.TxnPool.GetNonceByTxnPool(addr160)
			if err != nil {
				myNonce = chain.DefaultLedger.Store.GetNonce(addr160)
			}

			if myNonce < addrNonceReply.Nonce {
				addrTxnRelpy, err := localNode.requestSyncAddrTxn(nbr, addrNonceReply.Address, myNonce)
				if err != nil {
					log.Info("localNode.requestSyncAddrTxn err: ", err)
				} else {
					// update txn pool
					for _, pbTxn := range addrTxnRelpy.Transactions {

						txn := &transaction.Transaction{Transaction: pbTxn}
						if localNode.ExistHash(txn.Hash()) {
							continue
						}
						localNode.TxnPool.AppendTxnPool(txn)
					}
				}

			}

		}(neighbor)
	}

}

// send neighbor a REQ_ADDR_NONCE request message, and wait for reply.
func (localNode *LocalNode) requestAddrNonce(remoteNode *node.RemoteNode) (*pb.AddrNonce, error) {

	msg, err := NewReqAddrNonceMessage()
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

	replyMsg := &pb.AddrNonce{}
	err = proto.Unmarshal(replyBytes, replyMsg)
	if err != nil {
		return nil, err
	}

	return replyMsg, nil
}

// send to a neighbor a REQ_SYNC_ADDR_TXN request message, and wait for reply with timeout
// return the reply messge RPL_SYNC_ADDR_TXN
func (localNode *LocalNode) requestSyncAddrTxn(remoteNode *node.RemoteNode, addr []byte, nonce uint64) (*pb.Transactions, error) {

	msg, err := NewReqSyncAddrTxnMessage(addr, nonce)
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

	replyMsg := &pb.Transactions{}
	err = proto.Unmarshal(replyBytes, replyMsg)
	if err != nil {
		return nil, err
	}

	log.Info("Sync txn by rand address, get ", len(replyMsg.Transactions), "txns from neighbor")

	return replyMsg, nil
}

// build a REQ_ADDR_NONCE request message
func NewReqAddrNonceMessage() (*pb.UnsignedMessage, error) {

	msg := &pb.UnsignedMessage{
		MessageType: pb.MessageType_REQ_ADDR_NONCE,
	}

	return msg, nil

}

// build RPL_ADDR_NONCE reply message
// reply with rand address my txn pool's with this address' nonce.
func NewRplAddrNonceMessage(tp *pool.TxnPool) (*pb.UnsignedMessage, error) {
	addrMap := tp.GetAddressList()
	addrCount := len(addrMap)

	var addr common.Uint160
	var nonce uint64
	if addrCount > 0 {
		mapKeys := reflect.ValueOf(addrMap).MapKeys()

		// get a rand address
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		rn := r.Intn(addrCount)
		addr = mapKeys[rn].Interface().(common.Uint160)

		var err error
		nonce, err = tp.GetNonceByTxnPool(addr) // here get expected nonce which is current last nonce + 1
		if err != nil {
			return nil, err
		}
	}

	msgBody := &pb.AddrNonce{
		Address: addr.ToArray(),
		Nonce:   nonce,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.MessageType_RPL_ADDR_NONCE,
		Message:     buf,
	}

	return msg, nil
}

// build REQ_SYNC_ADDR_TXN request message
// This message includse my txn pool's txn address and maximum nonce.
func NewReqSyncAddrTxnMessage(addr []byte, nonce uint64) (*pb.UnsignedMessage, error) {

	msgBody := &pb.AddrNonce{Address: addr, Nonce: nonce}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.MessageType_REQ_SYNC_ADDR_TXN,
		Message:     buf,
	}

	return msg, nil

}

// build RPL_SYNC_ADDR_TXN message. reply transactions which are needed to sync to the requestor.
func NewRplSyncAddrTxnMessage(reqMsg *pb.AddrNonce, tp *pool.TxnPool) (*pb.UnsignedMessage, error) {

	addr160, err := common.Uint160ParseFromBytes(reqMsg.Address)
	if err != nil {
		return nil, err
	}

	// here list is sorted, ordered by nonce
	list := tp.GetAllTransactionsBySender(addr160)

	i := 0
	for i = 0; i < len(list); i++ {
		if list[i].UnsignedTx.Nonce >= reqMsg.Nonce {
			break
		}
	}

	var respTxns []*pb.Transaction
	for ; i < len(list); i++ {
		respTxns = append(respTxns, list[i].Transaction)
	}

	msgBody := &pb.Transactions{Transactions: respTxns}

	log.Info("Sync txn by rand address, send ", len(respTxns), " txns to neighbor")

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.MessageType_RPL_SYNC_ADDR_TXN,
		Message:     buf,
	}

	return msg, nil
}

// handle a REQ_ADDR_NONCE request message. return a RPL_ADDR_NONCE response message buffer.
func (localNode *LocalNode) requestAddrNonceHandler(remoteMessage *node.RemoteMessage) ([]byte, bool, error) {

	replyMsg, err := NewRplAddrNonceMessage(localNode.TxnPool)
	if err != nil {
		return nil, false, err
	}

	replyBuf, err := localNode.SerializeMessage(replyMsg, false)
	return replyBuf, false, err
}

// Handle REQ_SYNC_ADDR_TXN request message, return RPL_SYNC_ADDR_TXN response message buffer.
func (localNode *LocalNode) requestSyncAddrTxnHandler(remoteMessage *node.RemoteMessage) ([]byte, bool, error) {

	reqMsg := &pb.AddrNonce{}
	err := proto.Unmarshal(remoteMessage.Message, reqMsg)
	if err != nil {
		return nil, false, err
	}

	replyMsg, err := NewRplSyncAddrTxnMessage(reqMsg, localNode.TxnPool)
	if err != nil {
		return nil, false, err
	}

	replyBuf, err := localNode.SerializeMessage(replyMsg, false)
	return replyBuf, false, err
}
