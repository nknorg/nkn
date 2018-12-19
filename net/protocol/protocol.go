package protocol

import (
	"bytes"
	"encoding/binary"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/pool"
	"github.com/nknorg/nkn/crypto"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/protobuf"
	"github.com/nknorg/nkn/vault"
)

// node connection state
const (
	INIT       = 0
	HAND       = 1
	HANDSHAKE  = 2
	HANDSHAKED = 3
	ESTABLISH  = 4
	INACTIVITY = 5
)

type SyncState byte

const (
	SyncStarted     SyncState = 0
	SyncFinished    SyncState = 1
	PersistFinished SyncState = 2
)

// SyncStateString is the string of SyncState enum
var SyncStateString = []string{"SyncStarted", "SyncFinished", "PersistFinished"}

type Noder interface {
	Version() uint32
	GetID() uint64
	GetAddr() string
	GetPort() uint16
	GetAddrStr() string
	GetAddr16() ([16]byte, error)
	GetHttpJsonPort() uint16
	//TODO SetHttpJsonPort(uint16)
	GetWsPort() uint16
	//TODO SetWsPort(uint16)
	GetSyncState() SyncState
	SetSyncState(s SyncState)
	SetSyncStopHash(hash Uint256, height uint32)
	SyncBlock(bool)
	SyncBlockMonitor(bool)
	StopSyncBlock(bool)
	GetPubKey() *crypto.PubKey
	LocalNode() Noder
	DelNbrNode(id uint64) (Noder, bool)
	AddNbrNode(Noder)
	CloseConn()
	GetHeight() uint32
	GetConnectionCnt() uint
	GetTxnByCount(int, Uint256) (map[Uint256]*transaction.Transaction, error)
	GetTxnPool() *pool.TxnPool
	AppendTxnPool(*transaction.Transaction) ErrCode
	ExistHash(hash Uint256) bool
	DumpInfo()
	Tx(buf []byte)
	GetTime() int64
	GetEvent(eventName string) *events.Event
	GetNeighborAddrs() ([]NodeAddr, uint)
	GetConnDirection() string
	GetTransaction(hash Uint256) *transaction.Transaction
	IncRxTxnCnt()
	GetTxnCnt() uint64
	GetRxTxnCnt() uint64

	Xmit(interface{}) error
	Broadcast([]byte) error
	GetNeighborHeights() ([]uint32, uint)
	CleanSubmittedTransactions(txns []*transaction.Transaction) error

	GetNeighborNoder() []Noder
	GetSyncFinishedNeighbors() []Noder
	StoreFlightHeight(height uint32)
	GetFlightHeightCnt() int
	RemoveFlightHeightLessThan(height uint32)
	RemoveFlightHeight(height uint32)
	SetHeight(height uint32)
	WaitForSyncBlkFinish()
	GetFlightHeights() []uint32

	GetChordAddr() []byte
	StartRelayer(wallet vault.Wallet)
	SendRelayPacket(srcAddr, destAddr string, payload, signature []byte, maxHoldingSeconds uint32) error
	SendRelayPacketsInBuffer(clientId []byte) error
	GetWsAddr() string
	FindWsAddr([]byte) (string, error)
	FindHttpProxyAddr([]byte) (string, error)
	FindSuccessorAddrs([]byte, int) ([]string, error)
	DumpChordInfo() *ChordInfo
	GetSuccessors() []*ChordNodeInfo
	GetPredecessors() []*ChordNodeInfo
	GetFingerTab() map[int][]*ChordNodeInfo
}

type ChordInfo struct {
	Node         ChordNodeInfo
	Successors   []*ChordNodeInfo
	Predecessors []*ChordNodeInfo
	FingerTable  map[int][]*ChordNodeInfo
}

type ChordNodeInfo struct {
	ID         string
	Addr       string
	IsOutbound bool
	protobuf.NodeData
}

type NodeAddr struct {
	Time    int64
	IpAddr  [16]byte
	IpStr   string
	InOut   string
	Port    uint16
	ID      uint64
	NKNaddr string
}

func (msg *NodeAddr) Deserialization(p []byte) error {
	buf := bytes.NewBuffer(p)
	err := binary.Read(buf, binary.LittleEndian, msg)
	return err
}

func (msg NodeAddr) Serialization() ([]byte, error) {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.LittleEndian, msg)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), err
}
