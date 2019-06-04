package node

import (
	"errors"
	"fmt"
	"hash/fnv"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/chain/pool"
	"github.com/nknorg/nkn/crypto/util"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/log"
	nnetpb "github.com/nknorg/nnet/protobuf"
)

const (
	requestTxnChanLen                = 1000
	requestTxnSaltSize               = 32
	requestSigChainTxnWorkerPoolSize = 10
)

type requestTxnInfo struct {
	neighborID string
	hash       []byte
}

type requestTxnChan chan *requestTxnInfo

type requestTxn struct {
	chans []requestTxnChan
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

func (rt *requestTxn) receiveSigChainTxnHash(neighborID string, sigHash []byte) error {
	ch := rt.getChan(sigHash)
	if ch == nil {
		return errors.New("request txn chan is nil")
	}

	info := &requestTxnInfo{
		neighborID: neighborID,
		hash:       sigHash,
	}

	select {
	case ch <- info:
	default:
		return errors.New("request txn chan full")
	}

	return nil
}

func (localNode *LocalNode) initTxnHandlers() {
	localNode.AddMessageHandler(pb.TRANSACTIONS, localNode.transactionsMessageHandler)
	localNode.AddMessageHandler(pb.I_HAVE_SIGNATURE_CHAIN_TRANSACTION, localNode.iHaveSignatureChainTransactionMessageHandler)
	localNode.AddMessageHandler(pb.REQUEST_SIGNATURE_CHAIN_TRANSACTION, localNode.requestSignatureChainTransactionMessageHandler)
}

func (localNode *LocalNode) startRequestingSigChainTxn() {
	for _, ch := range localNode.requestSigChainTxn.chans {
		go func(ch requestTxnChan) {
			var info *requestTxnInfo
			var neighbor *RemoteNode
			var txn *transaction.Transaction
			var porPkg *por.PorPackage
			var err error
			for {
				info = <-ch

				if txn, err = por.GetPorServer().GetSigChainTxnBySigHash(info.hash); err == nil && txn != nil {
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

				err = chain.VerifyTransaction(txn)
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

// transactionsMessageHandler handles a TRANSACTIONS message
func (localNode *LocalNode) transactionsMessageHandler(remoteMessage *RemoteMessage) ([]byte, bool, error) {
	msgBody := &pb.Transactions{}
	err := proto.Unmarshal(remoteMessage.Message, msgBody)
	if err != nil {
		return nil, false, err
	}

	if len(msgBody.Transactions) == 0 {
		return nil, false, fmt.Errorf("no transactions in message body")
	}

	hasValidTxn := false
	shouldPropagate := false
	for _, msgTxn := range msgBody.Transactions {
		txn := &transaction.Transaction{Transaction: msgTxn}

		if localNode.ExistHash(txn.Hash()) {
			hasValidTxn = true
			continue
		}

		errCode := localNode.AppendTxnPool(txn)
		if errCode == pool.ErrDuplicatedTx {
			hasValidTxn = true
			continue
		}
		if errCode != nil {
			log.Warningf("Verify transaction failed with %v when append to txn pool", errCode)
			continue
		}

		hasValidTxn = true
		shouldPropagate = true
	}

	if !hasValidTxn {
		return nil, false, fmt.Errorf("all transactions in msg are invalid")
	}

	if !shouldPropagate {
		return nil, false, errors.New("Do Not Propagate")
	}

	return nil, false, nil
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

	err = localNode.requestSigChainTxn.receiveSigChainTxnHash(remoteMessage.Sender.GetID(), msgBody.SignatureHash)
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
	return txn, nil
}

func (localNode *LocalNode) cleanupTransactions(v interface{}) {
	if block, ok := v.(*block.Block); ok {
		localNode.TxnPool.CleanSubmittedTransactions(block.Transactions)
	}
}
