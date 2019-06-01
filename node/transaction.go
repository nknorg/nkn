package node

import (
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/chain/pool"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/log"
	nnetpb "github.com/nknorg/nnet/protobuf"
)

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
		if errCode == pool.ErrNonOptimalSigChain || errCode == pool.ErrDuplicatedTx {
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

	if txn.UnsignedTx.Payload.Type == pb.CommitType {
		_, err = localNode.nnet.SendBytesBroadcastAsync(buf, nnetpb.BROADCAST_PUSH)
	} else {
		_, err = localNode.nnet.SendBytesBroadcastAsync(buf, nnetpb.BROADCAST_TREE)
	}
	if err != nil {
		return err
	}

	localNode.ExistHash(txn.Hash())

	return nil
}

func (localNode *LocalNode) cleanupTransactions(v interface{}) {
	if block, ok := v.(*block.Block); ok {
		localNode.TxnPool.CleanSubmittedTransactions(block.Transactions)
	}
}
