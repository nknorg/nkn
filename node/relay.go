package node

import (
	"fmt"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/event"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/config"
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

func (rs *RelayService) Start() error {
	event.Queue.Subscribe(event.NewBlockProduced, rs.populateVRFCache)
	event.Queue.Subscribe(event.NewBlockProduced, rs.flushSigChain)
	event.Queue.Subscribe(event.BacktrackSigChain, rs.backtrackDestSigChain)
	rs.localNode.AddMessageHandler(pb.RELAY, rs.relayMessageHandler)
	rs.localNode.AddMessageHandler(pb.BACKTRACK_SIGNATURE_CHAIN, rs.backtrackSigChainMessageHandler)
	return nil
}

// NewRelayMessage creates a RELAY message
func NewRelayMessage(srcIdentifier string, srcPubkey, destID, payload, blockHash, signature []byte, maxHoldingSeconds uint32) (*pb.UnsignedMessage, error) {
	msgBody := &pb.Relay{
		SrcIdentifier:     srcIdentifier,
		SrcPubkey:         srcPubkey,
		DestId:            destID,
		Payload:           payload,
		MaxHoldingSeconds: maxHoldingSeconds,
		BlockHash:         blockHash,
		LastSignature:     signature,
		SigChainLen:       1,
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

	event.Queue.Notify(event.SendInboundMessageToClient, msgBody)

	return nil, false, nil
}

// NewBacktrackSigChainMessage creates a BACKTRACK_SIGNATURE_CHAIN message
func NewBacktrackSigChainMessage(sigChainElems []*pb.SigChainElem, prevSignature []byte) (*pb.UnsignedMessage, error) {
	msgBody := &pb.BacktrackSignatureChain{
		SigChainElems: sigChainElems,
		PrevSignature: prevSignature,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.BACKTRACK_SIGNATURE_CHAIN,
		Message:     buf,
	}

	return msg, nil
}

// backtrackSigChainMessageHandler handles a BACKTRACK_SIGNATURE_CHAIN message
func (rs *RelayService) backtrackSigChainMessageHandler(remoteMessage *RemoteMessage) ([]byte, bool, error) {
	msgBody := &pb.BacktrackSignatureChain{}
	err := proto.Unmarshal(remoteMessage.Message, msgBody)
	if err != nil {
		return nil, false, err
	}

	err = rs.backtrackSigChain(msgBody.SigChainElems, msgBody.PrevSignature, remoteMessage.Sender.PublicKey)
	if err != nil {
		return nil, false, err
	}

	return nil, false, nil
}

func (rs *RelayService) backtrackSigChain(sigChainElems []*pb.SigChainElem, signature, senderPubkey []byte) error {
	sigChainElems, prevSignature, prevNodeID, err := rs.porServer.BacktrackSigChain(sigChainElems, signature, senderPubkey)
	if err != nil {
		return err
	}

	if prevNodeID == nil {
		var sigChain *pb.SigChain
		sigChain, err = rs.porServer.GetSrcSigChainFromCache(prevSignature)
		if err != nil {
			return err
		}

		sigChain.Elems = append(sigChain.Elems, sigChainElems...)

		err = rs.broadcastSigChain(sigChain)
		if err != nil {
			return err
		}

		return nil
	}

	nextHop := rs.localNode.GetNbrNode(chordIDToNodeID(prevNodeID))
	if nextHop == nil {
		return fmt.Errorf("cannot find next hop with id %x", prevNodeID)
	}

	nextMsg, err := NewBacktrackSigChainMessage(sigChainElems, prevSignature)
	if err != nil {
		return err
	}

	buf, err := rs.localNode.SerializeMessage(nextMsg, false)
	if err != nil {
		return err
	}

	err = nextHop.SendBytesAsync(buf)
	if err != nil {
		return err
	}

	return nil
}

func (rs *RelayService) broadcastSigChain(sigChain *pb.SigChain) error {
	buf, err := proto.Marshal(sigChain)
	if err != nil {
		return err
	}

	txn, err := MakeCommitTransaction(rs.wallet, buf)
	if err != nil {
		return err
	}

	err = chain.VerifyTransaction(txn)
	if err != nil {
		return err
	}

	porPkg, err := por.GetPorServer().AddSigChainFromTx(txn, chain.DefaultLedger.Store.GetHeight())
	if err != nil {
		return err
	}
	if porPkg == nil {
		return nil
	}

	err = rs.localNode.iHaveSignatureChainTransaction(porPkg.VoteForHeight, porPkg.SigHash, nil)
	if err != nil {
		return err
	}

	return nil
}

func (rs *RelayService) backtrackDestSigChain(v interface{}) {
	sigChainInfo, ok := v.(*por.BacktrackSigChainInfo)
	if !ok {
		log.Error("Decode backtrack sigchain info failed")
		return
	}

	err := rs.backtrackSigChain(
		[]*pb.SigChainElem{sigChainInfo.DestSigChainElem},
		sigChainInfo.PrevSignature,
		nil,
	)
	if err != nil {
		log.Errorf("Backtrack sigchain error: %v", err)
	}
}

func (rs *RelayService) signRelayMessage(relayMessage *pb.Relay, nextHop, prevHop *RemoteNode) error {
	var nextPubkey []byte
	var err error
	if nextHop != nil {
		nextPubkey, err = nextHop.GetPubKey().EncodePoint(true)
		if err != nil {
			return err
		}
	}

	mining := config.Parameters.Mining && rs.localNode.GetSyncState() == pb.PersistFinished

	var prevNodeID []byte
	if prevHop != nil {
		prevNodeID = prevHop.Id
	}

	return rs.porServer.Sign(relayMessage, nextPubkey, prevNodeID, mining)
}

func (localNode *LocalNode) startRelayer() {
	localNode.relayer.Start()
}

func (localNode *LocalNode) SendRelayMessage(srcAddr, destAddr string, payload, signature []byte, maxHoldingSeconds uint32) error {
	srcID, srcPubkey, srcIdentifier, err := address.ParseClientAddress(srcAddr)
	if err != nil {
		return err
	}

	destID, destPubkey, _, err := address.ParseClientAddress(destAddr)
	if err != nil {
		return err
	}

	height := chain.DefaultLedger.Store.GetHeight() - config.MaxRollbackBlocks
	if height < 0 {
		height = 0
	}
	blockHash := chain.DefaultLedger.Store.GetHeaderHashByHeight(height)
	_, err = por.GetPorServer().CreateSigChainForClient(
		uint32(len(payload)),
		&blockHash,
		srcID,
		srcPubkey,
		destID,
		destPubkey,
		signature,
		pb.VRF,
	)
	if err != nil {
		return err
	}

	msg, err := NewRelayMessage(srcIdentifier, srcPubkey, destID, payload, blockHash.ToArray(), signature, maxHoldingSeconds)
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

func MakeCommitTransaction(wallet vault.Wallet, sigChain []byte) (*transaction.Transaction, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	//TODO modify nonce
	txn, err := transaction.NewCommitTransaction(sigChain, account.ProgramHash, 0)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func (rs *RelayService) populateVRFCache(v interface{}) {
	block, ok := v.(*block.Block)
	if !ok {
		return
	}

	blockHash := block.Hash()
	rs.porServer.GetOrComputeVrf(blockHash.ToArray())
}

func (rs *RelayService) flushSigChain(v interface{}) {
	block, ok := v.(*block.Block)
	if !ok {
		return
	}

	height := block.Header.UnsignedHeader.Height - config.MaxRollbackBlocks - 1
	if height < 0 {
		height = 0
	}
	blockHash := chain.DefaultLedger.Store.GetHeaderHashByHeight(height)

	rs.porServer.FlushSigChain(blockHash.ToArray())
}
