// interface of local node
package node

import (
	"github.com/nknorg/nkn/v2/chain/pool"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/transaction"
	"github.com/nknorg/nnet"
	nnetnode "github.com/nknorg/nnet/node"
)

<<<<<<< HEAD
=======
// for encryption
const (
	NonceSize     = 24
	SharedKeySize = 32
)

>>>>>>> 595e7a737e474cb689dd655dd14db03bd4e1d517
type IChordInfo interface{}

type ILocalNode interface {

	// info
	GetNeighborInfo() []*RemoteNode
	GetChordInfo() IChordInfo

	// node
	GetMinVerifiableHeight() uint32
	GetChordID() []byte

	// handler
	AddMessageHandler(messageType pb.MessageType, handler MessageHandler)

	SerializeMessage(unsignedMsg *pb.UnsignedMessage, sign bool) ([]byte, error)

	// localNode
	MarshalJSON() ([]byte, error)
	Start() error
	GetProposalSubmitted() uint32
	IncrementProposalSubmitted()
	GetRelayMessageCount() uint64
	IncrementRelayMessageCount()
	GetTxnPool() *pool.TxnPool
	GetHeight() uint32
	SetSyncState(s pb.SyncState) bool
	GetSyncState() pb.SyncState
	SetMinVerifiableHeight(height uint32)
	GetWsAddr() string
	GetWssAddr() string
	FindSuccessorAddrs(key []byte, numSucc int) ([]string, error)
	FindWsAddr(key []byte) (string, string, []byte, []byte, error)
	FindWssAddr(key []byte) (string, string, []byte, []byte, error)
	CheckIDChange(v interface{})
	ComputeSharedKey(remotePublicKey []byte) (*[SharedKeySize]byte, error)
	GetNnet() *nnet.NNet

	// neighbor
	GetNeighborByNNetNode(nnetRemoteNode *nnetnode.RemoteNode) *RemoteNode
	GetGossipNeighbors(filter func(*RemoteNode) bool) []*RemoteNode
	GetVotingNeighbors(filter func(*RemoteNode) bool) []*RemoteNode
	VerifySigChain(sc *pb.SigChain, height uint32) error
	VerifySigChainObjection(sc *pb.SigChain, reporterID []byte, height uint32) (int, error)
	GetNeighborNode(id string) *RemoteNode
	RemoveNeighborNode(id string)

	// relay
	SendRelayMessage(srcAddr, destAddr string, payload, signature, blockHash []byte, nonce, maxHoldingSeconds uint32) error
	NewSignatureChainObjectionMessage(height uint32, sigHash []byte) (*pb.UnsignedMessage, error)
	StartSyncing(syncStopHash common.Uint256, syncStopHeight uint32, neighbors []*RemoteNode) (bool, error)
	ResetSyncing()
	BroadcastTransaction(txn *transaction.Transaction) error

	// neighborNodes
	GetConnectionCnt() uint
	GetNeighbors(filter func(*RemoteNode) bool) []*RemoteNode

	// txnpool
	AppendTxnPool(txn *transaction.Transaction) error
}
