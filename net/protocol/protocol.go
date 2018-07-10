package protocol

import (
	"bytes"
	"encoding/binary"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/pool"
	"github.com/nknorg/nkn/crypto"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/chord"
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

type Noder interface {
	Version() uint32
	GetID() uint64
	Services() uint64
	GetAddr() string
	GetPort() uint16
	GetHttpInfoPort() uint16
	SetHttpInfoPort(uint16)
	GetHttpJsonPort() uint16
	//TODO SetHttpJsonPort(uint16)
	GetWebSockPort() uint16
	//TODO SetWebSockPort(uint16)
	GetChordPort() uint16
	//TODO SetChordPort(uint16)
	GetHttpInfoState() bool
	SetHttpInfoState(bool)
	GetState() uint32
	SetState(state uint32)
	GetSyncState() SyncState
	SetSyncState(s SyncState)
	SetSyncStopHash(hash Uint256, height uint32)
	SyncBlock(bool)
	SyncBlockMonitor(bool)
	GetRelay() bool
	GetPubKey() *crypto.PubKey
	CompareAndSetState(old, new uint32) bool
	UpdateRXTime(t time.Time)
	LocalNode() Noder
	DelNbrNode(id uint64) (Noder, bool)
	AddNbrNode(Noder)
	CloseConn()
	GetHeight() uint32
	GetConnectionCnt() uint
	GetTxnByCount(int) map[Uint256]*transaction.Transaction
	GetTxnPool() *pool.TxnPool
	AppendTxnPool(*transaction.Transaction) ErrCode
	ExistedID(id Uint256) bool
	ReqNeighborList()
	DumpInfo()
	UpdateInfo(t time.Time, version uint32, services uint64, port uint16, nonce uint64, relay uint8, height uint32)
	ConnectNeighbors()
	Connect(nodeAddr string) error
	Tx(buf []byte)
	GetTime() int64
	NodeEstablished(uid uint64) bool
	GetEvent(eventName string) *events.Event
	GetNeighborAddrs() ([]NodeAddr, uint64)
	GetTransaction(hash Uint256) *transaction.Transaction
	IncRxTxnCnt()
	GetTxnCnt() uint64
	GetRxTxnCnt() uint64

	Xmit(interface{}) error
	GetBookKeeperAddr() *crypto.PubKey
	GetBookKeepersAddrs() ([]*crypto.PubKey, uint64)
	SetBookKeeperAddr(pk *crypto.PubKey)
	GetNeighborHeights() ([]uint32, uint64)
	CleanSubmittedTransactions(txns []*transaction.Transaction) error

	GetNeighborNoder() []Noder
	GetSyncFinishedNeighbors() []Noder
	GetNbrNodeCnt() uint32
	StoreFlightHeight(height uint32)
	GetFlightHeightCnt() int
	RemoveFlightHeightLessThan(height uint32)
	RemoveFlightHeight(height uint32)
	GetLastRXTime() time.Time
	SetHeight(height uint32)
	WaitForSyncBlkFinish()
	GetFlightHeights() []uint32
	IsAddrInNbrList(addr string) bool
	SetAddrInConnectingList(addr string) bool
	RemoveAddrInConnectingList(addr string)
	AddInRetryList(addr string)
	RemoveFromRetryList(addr string)
	AcqSyncReqSem()
	RelSyncReqSem()

	GetChordAddr() []byte
	GetChordRing() *chord.Ring
	StartRelayer(wallet vault.Wallet)
	NextHop(key []byte) (Noder, error)
	SendRelayPacket(srcID, srcPubkey, destID, destPubkey, payload, signature []byte) error
	SendRelayPacketsInBuffer(clientId []byte) error
}

type NodeAddr struct {
	Time     int64
	Services uint64
	IpAddr   [16]byte
	Port     uint16
	ID       uint64
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
