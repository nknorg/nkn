package node

import (
	"crypto/sha256"
	"errors"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/chain/pool"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	. "github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
	"github.com/nknorg/nkn/vm/contract"
)

type RelayService struct {
	sync.Mutex
	wallet    vault.Wallet
	localNode *LocalNode
	porServer *por.PorServer
}

func NewRelayService(wallet vault.Wallet, localNode *LocalNode) *RelayService {
	service := &RelayService{
		wallet:    wallet,
		localNode: localNode,
		porServer: por.GetPorServer(),
	}
	return service
}

// NewRelayMessage creates a RELAY message
func NewRelayMessage(srcAddr string, destID, payload []byte, sigChain *pb.SigChain, maxHoldingSeconds uint32) (*pb.UnsignedMessage, error) {
	msgBody := &pb.Relay{
		SrcAddr:           srcAddr,
		DestId:            destID,
		Payload:           payload,
		SigChain:          sigChain,
		MaxHoldingSeconds: maxHoldingSeconds,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.RELAY,
		Message:     buf,
	}

	return msg, nil
}

// relayMessageHandler handles a RELAY message
func (rs *RelayService) relayMessageHandler(remoteMessage *RemoteMessage) ([]byte, bool, error) {
	msgBody := &pb.Relay{}
	err := proto.Unmarshal(remoteMessage.Message, msgBody)
	if err != nil {
		return nil, false, err
	}

	rs.localNode.GetEvent("relay").Notify(events.EventSendInboundMessageToClient, msgBody)

	return nil, false, nil
}

func (rs *RelayService) Start() error {
	rs.localNode.GetEvent("relay").Subscribe(events.EventReceiveClientSignedSigChain, rs.receiveClientSignedSigChainNoError)
	rs.localNode.AddMessageHandler(pb.RELAY, rs.relayMessageHandler)
	return nil
}

func (rs *RelayService) signRelayMessage(relayMessage *pb.Relay, nextHop *RemoteNode) error {
	var nextPubkey []byte
	var err error
	if nextHop != nil {
		nextPubkey, err = nextHop.GetPubKey().EncodePoint(true)
		if err != nil {
			return err
		}
	} else {
		nextPubkey = relayMessage.SigChain.GetDestPubkey()
	}

	mining := rs.localNode.GetSyncState() == pb.PersistFinished

	return rs.porServer.Sign(relayMessage.SigChain, nextPubkey, mining)
}

func (rs *RelayService) receiveClientSignedSigChain(v interface{}) error {
	relayMessage, ok := v.(*pb.Relay)
	if !ok {
		return errors.New("Decode client signed sigchain failed")
	}

	// TODO: client should sign this last piece
	_, err := relayMessage.SigChain.ExtendElement(relayMessage.DestId, relayMessage.SigChain.GetDestPubkey(), false)
	if err != nil {
		return err
	}

	// TODO: only pick sigchain to sign when threshold is smaller than
	buf, err := proto.Marshal(relayMessage.SigChain)
	if err != nil {
		return err
	}

	txn, err := MakeCommitTransaction(rs.wallet, buf)
	if err != nil {
		return err
	}

	errCode := rs.localNode.AppendTxnPool(txn)
	if errCode == pool.ErrNonOptimalSigChain {
		return nil
	}
	if errCode != nil {
		return errCode
	}

	err = rs.localNode.BroadcastTransaction(txn)
	if err != nil {
		return err
	}

	return nil
}

func (rs *RelayService) receiveClientSignedSigChainNoError(v interface{}) {
	err := rs.receiveClientSignedSigChain(v)
	if err != nil {
		log.Error(err)
	}
}

func (localNode *LocalNode) startRelayer() {
	localNode.relayer.Start()
}

func (localNode *LocalNode) SendRelayMessage(srcAddr, destAddr string, payload, signature []byte, maxHoldingSeconds uint32) error {
	srcID, srcPubkey, err := address.ParseClientAddress(srcAddr)
	if err != nil {
		return err
	}

	destID, destPubkey, err := address.ParseClientAddress(destAddr)
	if err != nil {
		return err
	}

	payloadHash := sha256.Sum256(payload)
	payloadHash256, err := common.Uint256ParseFromBytes(payloadHash[:])
	if err != nil {
		return err
	}

	height := chain.DefaultLedger.Store.GetHeight() - por.SigChainBlockHeightOffset
	if height < 0 {
		height = 0
	}
	blockHash := chain.DefaultLedger.Store.GetHeaderHashByHeight(height)
	sigChain, err := por.GetPorServer().CreateSigChainForClient(
		uint32(len(payload)),
		&payloadHash256,
		&blockHash,
		srcID,
		srcPubkey,
		destPubkey,
		signature,
		pb.ED25519,
	)
	if err != nil {
		return err
	}

	msg, err := NewRelayMessage(srcAddr, destID, payload, sigChain, maxHoldingSeconds)
	if err != nil {
		return err
	}

	buf, err := localNode.SerializeMessage(msg, false)
	if err != nil {
		return err
	}

	_, err = localNode.nnet.SendBytesRelayAsync(buf, destID)
	if err != nil {
		return err
	}

	return nil
}

func MakeCommitTransaction(wallet vault.Wallet, sigChain []byte) (*Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	//TODO modify nonce
	txn, err := NewCommitTransaction(sigChain, account.ProgramHash, 0)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}
