package node

import (
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/chain/pool"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto/util"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	nnetnode "github.com/nknorg/nnet/node"
	nnetpb "github.com/nknorg/nnet/protobuf"
)

const (
	requestTxnSaltSize                  = 32
	requestTxnChanLen                   = 1000
	requestSigChainTxnWorkerPoolSize    = 10
	requestSigChainCacheExpiration      = 50 * config.ConsensusTimeout
	requestSigChainCacheCleanupInterval = config.ConsensusDuration
	receiveTxnMsgSaltSize               = 32
	receiveTxnMsgChanLen                = 10000
	receiveTxnMsgWorkerPoolSize         = 1
)

type requestTxnInfo struct {
	neighborID string
	hash       []byte
	height     uint32
}

type requestTxnChan chan *requestTxnInfo

type requestTxn struct {
	chans []requestTxnChan
	salt  []byte
}

type receiveTxnMsgInfo struct {
	txnMsg          *pb.Transactions
	remoteMessage   *nnetnode.RemoteMessage
	nextRemoteNodes []*nnetnode.RemoteNode
}

type receiveTxnMsgChan chan *receiveTxnMsgInfo

type receiveTxnMsg struct {
	chans []receiveTxnMsgChan
	salt  []byte
}

func newRequestTxn(size int, salt []byte) *requestTxn {
	if salt == nil {
		salt = util.RandomBytes(requestTxnSaltSize)
	}
	chans := make([]requestTxnChan, size)
	for i := range chans {
		chans[i] = make(requestTxnChan, requestTxnChanLen)
	}
	return &requestTxn{
		chans: chans,
		salt:  salt,
	}
}

func (rt *requestTxn) getChan(hash []byte) requestTxnChan {
	if len(rt.chans) == 0 {
		return nil
	}

	h := fnv.New32()
	h.Write(hash)
	h.Write(rt.salt)
	idx := h.Sum32() % uint32(len(rt.chans))
	return rt.chans[idx]
}

func (rt *requestTxn) receiveSigChainTxnHash(neighborID string, height uint32, sigHash []byte) error {
	ch := rt.getChan(sigHash)
	if ch == nil {
		return errors.New("request txn chan is nil")
	}

	info := &requestTxnInfo{
		neighborID: neighborID,
		hash:       sigHash,
		height:     height,
	}

	select {
	case ch <- info:
	default:
		return errors.New("request txn chan full")
	}

	return nil
}

func newReceiveTxnMsg(size int, salt []byte) *receiveTxnMsg {
	if salt == nil {
		salt = util.RandomBytes(receiveTxnMsgSaltSize)
	}
	chans := make([]receiveTxnMsgChan, size)
	for i := range chans {
		chans[i] = make(receiveTxnMsgChan, receiveTxnMsgChanLen)
	}
	return &receiveTxnMsg{
		chans: chans,
		salt:  salt,
	}
}

func (rt *receiveTxnMsg) getChan(hash []byte) receiveTxnMsgChan {
	if len(rt.chans) == 0 {
		return nil
	}

	h := fnv.New32()
	h.Write(hash)
	h.Write(rt.salt)
	idx := h.Sum32() % uint32(len(rt.chans))
	return rt.chans[idx]
}

func (rt *receiveTxnMsg) receiveTxnMsg(txnMsg *pb.Transactions, remoteMessage *nnetnode.RemoteMessage, nextRemoteNodes []*nnetnode.RemoteNode) error {
	ch := rt.getChan(remoteMessage.Msg.MessageId)
	if ch == nil {
		return errors.New("receive txn chan is nil")
	}

	info := &receiveTxnMsgInfo{
		txnMsg:          txnMsg,
		remoteMessage:   remoteMessage,
		nextRemoteNodes: nextRemoteNodes,
	}

	select {
	case ch <- info:
	default:
		return errors.New("receive txn chan full")
	}

	return nil
}

func (localNode *LocalNode) initTxnHandlers() {
	localNode.AddMessageHandler(pb.I_HAVE_SIGNATURE_CHAIN_TRANSACTION, localNode.iHaveSignatureChainTransactionMessageHandler)
	localNode.AddMessageHandler(pb.REQUEST_SIGNATURE_CHAIN_TRANSACTION, localNode.requestSignatureChainTransactionMessageHandler)
	localNode.startRequestingSigChainTxn()
	localNode.startReceivingTxnMsg()
}

func (localNode *LocalNode) startRequestingSigChainTxn() {
	requestedHashCache := common.NewGoCache(requestSigChainCacheExpiration, requestSigChainCacheCleanupInterval)
	for _, ch := range localNode.requestSigChainTxn.chans {
		go func(ch requestTxnChan) {
			var info *requestTxnInfo
			var neighbor *RemoteNode
			var txn *transaction.Transaction
			var porPkg *por.PorPackage
			var ok bool
			var err error
			for {
				info = <-ch

				if _, ok = requestedHashCache.Get(info.hash); ok {
					continue
				}

				if txn, err = por.GetPorServer().GetSigChainTxnBySigHash(info.hash); err == nil && txn != nil {
					continue
				}

				currentHeight := chain.DefaultLedger.Store.GetHeight()

				if !por.GetPorServer().ShouldAddSigChainToCache(currentHeight, info.height, info.hash) {
					continue
				}

				neighbor = localNode.GetNbrNode(info.neighborID)
				if neighbor == nil {
					continue
				}

				txn, err = localNode.requestSignatureChainTransaction(neighbor, info.hash)
				if err != nil {
					log.Warningf("Request sigchain txn error: %v", err)
					continue
				}

				requestedHashCache.Set(info.hash, struct{}{})

				err = chain.VerifyTransaction(txn, currentHeight+1)
				if err != nil {
					log.Warningf("Verify sigchain txn error: %v", err)
					continue
				}

				porPkg, err = por.GetPorServer().AddSigChainFromTx(txn, chain.DefaultLedger.Store.GetHeight())
				if err != nil {
					log.Warningf("Add sigchain from txn error: %v", err)
					continue
				}
				if porPkg == nil {
					continue
				}

				err = localNode.iHaveSignatureChainTransaction(porPkg.VoteForHeight, porPkg.SigHash, &info.neighborID)
				if err != nil {
					log.Warningf("Send I have sigchain txn error: %v", err)
					continue
				}
			}
		}(ch)
	}
}

func (localNode *LocalNode) startReceivingTxnMsg() {
	for _, ch := range localNode.receiveTxnMsg.chans {
		go func(ch receiveTxnMsgChan) {
			var info *receiveTxnMsgInfo
			var remoteNode *nnetnode.RemoteNode
			var shouldPropagate bool
			var err error
			for {
				info = <-ch

				shouldPropagate, err = localNode.handleTransactionsMessage(info.txnMsg)
				if err != nil {
					log.Warningf("Handle transactions msg error: %v", err)
					continue
				}

				if !shouldPropagate {
					continue
				}

				for _, remoteNode = range info.nextRemoteNodes {
					_, err = remoteNode.SendMessage(info.remoteMessage.Msg, false, 0)
					if err != nil {
						log.Warningf("Sending txn msg to neighbor %v error: %v", remoteNode, err)
						continue
					}
				}
			}
		}(ch)
	}
}

// NewTransactionsMessage creates a TRANSACTIONS message
func NewTransactionsMessage(transactions []*transaction.Transaction) (*pb.UnsignedMessage, error) {
	msgTransactions := make([]*pb.Transaction, len(transactions))
	for i, transaction := range transactions {
		msgTransactions[i] = transaction.Transaction
	}

	msgBody := &pb.Transactions{
		Transactions: msgTransactions,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.TRANSACTIONS,
		Message:     buf,
	}

	return msg, nil
}

// NewIHaveSignatureChainTransactionMessage creates a
// I_HAVE_SIGNATURE_CHAIN_TRANSACTION message
func NewIHaveSignatureChainTransactionMessage(height uint32, sigHash []byte) (*pb.UnsignedMessage, error) {
	msgBody := &pb.IHaveSignatureChainTransaction{
		Height:        height,
		SignatureHash: sigHash,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.I_HAVE_SIGNATURE_CHAIN_TRANSACTION,
		Message:     buf,
	}

	return msg, nil
}

// NewRequestSignatureChainTransactionMessage creates a
// REQUEST_SIGNATURE_CHAIN_TRANSACTION message
func NewRequestSignatureChainTransactionMessage(sigHash []byte) (*pb.UnsignedMessage, error) {
	msgBody := &pb.RequestSignatureChainTransaction{
		SignatureHash: sigHash,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.REQUEST_SIGNATURE_CHAIN_TRANSACTION,
		Message:     buf,
	}

	return msg, nil
}

// NewRequestSignatureChainTransactionReply creates a
// REQUEST_SIGNATURE_CHAIN_TRANSACTION_REPLY message
func NewRequestSignatureChainTransactionReply(transaction *transaction.Transaction) (*pb.UnsignedMessage, error) {
	msgBody := &pb.RequestSignatureChainTransactionReply{}
	if transaction != nil {
		msgBody.Transaction = transaction.Transaction
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.REQUEST_SIGNATURE_CHAIN_TRANSACTION_REPLY,
		Message:     buf,
	}

	return msg, nil
}

// handleTransactionsMessage will stop localNode from relaying msg if ANY of the
// txn in msg is in cache, ledger, or invalid. This is to prevent attacker from
// mixing valid and invalid txn in the same message.
func (localNode *LocalNode) handleTransactionsMessage(txnMsg *pb.Transactions) (bool, error) {
	if len(txnMsg.Transactions) == 0 {
		return false, fmt.Errorf("no transactions in message body")
	}

	for _, msgTxn := range txnMsg.Transactions {
		txn := &transaction.Transaction{Transaction: msgTxn}

		if localNode.ExistHash(txn.Hash()) {
			return false, nil
		}

		err := localNode.AppendTxnPool(txn)
		if err == pool.ErrDuplicatedTx || err == pool.ErrRejectLowPriority {
			return false, nil
		}
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

// iHaveSignatureChainTransactionMessageHandler handles a
// I_HAVE_SIGNATURE_CHAIN_TRANSACTION message
func (localNode *LocalNode) iHaveSignatureChainTransactionMessageHandler(remoteMessage *RemoteMessage) ([]byte, bool, error) {
	msgBody := &pb.IHaveSignatureChainTransaction{}
	err := proto.Unmarshal(remoteMessage.Message, msgBody)
	if err != nil {
		return nil, false, err
	}

	if !por.GetPorServer().ShouldAddSigChainToCache(chain.DefaultLedger.Store.GetHeight(), msgBody.Height, msgBody.SignatureHash) {
		return nil, false, nil
	}

	err = localNode.requestSigChainTxn.receiveSigChainTxnHash(remoteMessage.Sender.GetID(), msgBody.Height, msgBody.SignatureHash)
	if err != nil {
		return nil, false, err
	}

	return nil, false, nil
}

// SignatureChainTransactionMessageHandler handles a
// REQUEST_SIGNATURE_CHAIN_TRANSACTION message
func (localNode *LocalNode) requestSignatureChainTransactionMessageHandler(remoteMessage *RemoteMessage) ([]byte, bool, error) {
	replyMsg, err := NewRequestSignatureChainTransactionReply(nil)
	if err != nil {
		return nil, false, err
	}

	replyBuf, err := localNode.SerializeMessage(replyMsg, false)
	if err != nil {
		return nil, false, err
	}

	msgBody := &pb.RequestSignatureChainTransaction{}
	err = proto.Unmarshal(remoteMessage.Message, msgBody)
	if err != nil {
		return replyBuf, false, err
	}

	txn, err := por.GetPorServer().GetSigChainTxnBySigHash(msgBody.SignatureHash)
	if err != nil {
		return replyBuf, false, err
	}

	replyMsg, err = NewRequestSignatureChainTransactionReply(txn)
	if err != nil {
		return replyBuf, false, err
	}

	replyBuf, err = localNode.SerializeMessage(replyMsg, false)
	if err != nil {
		return replyBuf, false, err
	}

	return replyBuf, false, nil
}

// BroadcastTransaction broadcast a transaction to the network using
// TRANSACTIONS message
func (localNode *LocalNode) BroadcastTransaction(txn *transaction.Transaction) error {
	msg, err := NewTransactionsMessage([]*transaction.Transaction{txn})
	if err != nil {
		return err
	}

	buf, err := localNode.SerializeMessage(msg, false)
	if err != nil {
		return err
	}

	_, err = localNode.nnet.SendBytesBroadcastAsync(buf, nnetpb.BROADCAST_TREE)
	if err != nil {
		return err
	}

	localNode.ExistHash(txn.Hash())

	return nil
}

// iHaveSignatureChainTransaction sends I_HAVE_SIGNATURE_CHAIN_TRANSACTION
// message to neighbors informing them node has a signature chain txn
func (localNode *LocalNode) iHaveSignatureChainTransaction(height uint32, sigHash []byte, excludeNeighborID *string) error {
	msg, err := NewIHaveSignatureChainTransactionMessage(height, sigHash)
	if err != nil {
		return err
	}

	buf, err := localNode.SerializeMessage(msg, false)
	if err != nil {
		return err
	}

	for _, neighbor := range localNode.GetNeighbors(nil) {
		if excludeNeighborID != nil && neighbor.GetID() == *excludeNeighborID {
			continue
		}
		err = neighbor.SendBytesAsync(buf)
		if err != nil {
			log.Errorf("Send vote to neighbor %v error: %v", neighbor, err)
			continue
		}
	}

	return nil
}

func (localNode *LocalNode) requestSignatureChainTransaction(neighbor *RemoteNode, sigHash []byte) (*transaction.Transaction, error) {
	msg, err := NewRequestSignatureChainTransactionMessage(sigHash)
	if err != nil {
		return nil, err
	}

	buf, err := localNode.SerializeMessage(msg, false)
	if err != nil {
		return nil, err
	}

	replyBytes, err := neighbor.SendBytesSync(buf)
	if err != nil {
		return nil, err
	}

	replyMsg := &pb.RequestSignatureChainTransactionReply{}
	err = proto.Unmarshal(replyBytes, replyMsg)
	if err != nil {
		return nil, err
	}

	if replyMsg.Transaction == nil {
		return nil, errors.New("get nil sigchain transaction response")
	}

	txn := &transaction.Transaction{Transaction: replyMsg.Transaction}

	err = txn.VerifySignature()
	if err != nil {
		return nil, err
	}

	porPkg, err := por.NewPorPackage(txn, false)
	if err != nil {
		return nil, fmt.Errorf("create por package from txn error: %v", err)
	}

	if !bytes.Equal(porPkg.SigHash, sigHash) {
		return nil, fmt.Errorf("returned sighash %x is different from expected %x", porPkg.SigHash, sigHash)
	}

	return txn, nil
}

func (localNode *LocalNode) cleanupTransactions(v interface{}) {
	if block, ok := v.(*block.Block); ok {
		if err := localNode.TxnPool.CleanSubmittedTransactions(block.Transactions); err != nil {
			log.Errorf("CleanSubmittedTransactions error: %v", err)
		}
	}
}
