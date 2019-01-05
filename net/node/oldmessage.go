package node

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	nknErrors "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/log"
)

const (
	MsgHdrLen      = 24
	MsgCmdLen      = 12
	MsgCmdOffset   = 4
	MsgChecksumLen = 4
	NetID          = 0x00000000
	MaxHdrCnt      = 500
	MaxInvHdrCnt   = 500
	HashLen        = 32
)

type Messenger interface {
	Verify([]byte) error
	Handle(*RemoteNode) error
	serialization.SerializableData
}

// The network communication message header
type msgHdr struct {
	Magic uint32
	//ID	 uint64
	CMD      [MsgCmdLen]byte // The message type
	Length   uint32
	Checksum [MsgChecksumLen]byte
}

// Alloc different message stucture
// @t the message name or type
// @len the message length only valid for varible length structure
//
// Return:
// @Messenger the Messenger interface
// @error  error code
// FixMe fix the ugly multiple return.
func AllocMsg(t string, length int) Messenger {
	switch t {
	case "msgheader":
		var msg msgHdr
		return &msg
	case "getheaders":
		var msg headersReq
		// TODO fill the header and type
		copy(msg.hdr.CMD[0:len(t)], t)
		return &msg
	case "headers":
		var msg blkHeader
		copy(msg.hdr.CMD[0:len(t)], t)
		return &msg
	case "getdata":
		var msg dataReq
		copy(msg.msgHdr.CMD[0:len(t)], t)
		return &msg
	case "block":
		var msg block
		copy(msg.msgHdr.CMD[0:len(t)], t)
		return &msg
	case "tx":
		var msg trn
		copy(msg.msgHdr.CMD[0:len(t)], t)
		return &msg
	case "notfound":
		var msg notFound
		copy(msg.msgHdr.CMD[0:len(t)], t)
		return &msg
	case "relay":
		var msg RelayMessage
		copy(msg.msgHdr.CMD[0:len(t)], t)
		return &msg
	default:
		log.Warning("Unknown message type")
		return nil
	}
}

func MsgType(buf []byte) (string, error) {
	cmd := buf[MsgCmdOffset : MsgCmdOffset+MsgCmdLen]
	n := bytes.IndexByte(cmd, 0)
	if n < 0 || n >= MsgCmdLen {
		return "", errors.New("Unexpected length of CMD command")
	}
	s := string(cmd[:n])
	return s, nil
}

func ParseMsg(buf []byte) (Messenger, error) {
	if len(buf) < MsgHdrLen {
		log.Warning("Unexpected size of received message")
		return nil, errors.New("Unexpected size of received message")
	}

	s, err := MsgType(buf)
	if err != nil {
		log.Error("Message type parsing error")
		return nil, err
	}

	msg := AllocMsg(s, len(buf))
	if msg == nil {
		log.Error(fmt.Sprintf("Allocation message %s failed", s))
		return nil, errors.New("Allocation message failed")
	}
	// Todo attach a node pointer to each message
	// Todo drop the message when verify/deseria packet error
	r := bytes.NewReader(buf[:])
	err = msg.Deserialize(r)
	if err != nil {
		log.Error("Deserialize error:", err)
		return nil, err
	}
	err = msg.Verify(buf[MsgHdrLen:])
	if err != nil {
		log.Error("Verify msg error:", err)
		return nil, err
	}

	return msg, nil
}

func HandleNodeMsg(node *RemoteNode, buf []byte) error {
	msg, err := ParseMsg(buf)
	if err != nil {
		return err
	}

	return msg.Handle(node)
}

func magicVerify(magic uint32) bool {
	return magic == NetID
}

func ValidMsgHdr(buf []byte) bool {
	var h msgHdr
	r := bytes.NewReader(buf)
	h.Deserialize(r)
	//TODO: verify hdr checksum
	return magicVerify(h.Magic)
}

func PayloadLen(buf []byte) int {
	var h msgHdr
	r := bytes.NewReader(buf)
	h.Deserialize(r)
	return int(h.Length)
}

func checkSum(p []byte) []byte {
	t := sha256.Sum256(p)
	s := sha256.Sum256(t[:])

	// Currently we only need the front 4 bytes as checksum
	return s[:MsgChecksumLen]
}

func (hdr *msgHdr) init(cmd string, checksum []byte, length uint32) {
	hdr.Magic = NetID
	copy(hdr.CMD[0:uint32(len(cmd))], cmd)
	copy(hdr.Checksum[:], checksum[:MsgChecksumLen])
	hdr.Length = length
	//hdr.ID = id
}

// Verify the message header information
// @p payload of the message
func (hdr msgHdr) Verify(buf []byte) error {
	if !magicVerify(hdr.Magic) {
		log.Warning(fmt.Sprintf("Unmatched magic number 0x%0x", hdr.Magic))
		return errors.New("Unmatched magic number")
	}
	checkSum := checkSum(buf)
	if !bytes.Equal(hdr.Checksum[:], checkSum[:]) {
		str1 := hex.EncodeToString(hdr.Checksum[:])
		str2 := hex.EncodeToString(checkSum[:])
		log.Warning(fmt.Sprintf("Message Checksum error, Received checksum %s Wanted checksum: %s",
			str1, str2))
		return errors.New("Message Checksum error")
	}

	return nil
}

func (msg *msgHdr) Deserialize(r io.Reader) error {
	return binary.Read(r, binary.LittleEndian, msg)
}

func (hdr msgHdr) Serialize(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, hdr)
}

func (hdr msgHdr) Handle(n *RemoteNode) error {
	// TBD
	return nil
}

type block struct {
	msgHdr
	blk ledger.Block
}

func (msg block) Handle(node *RemoteNode) error {
	hash := msg.blk.Hash()
	if ledger.DefaultLedger.BlockInLedger(hash) {
		log.Debug("Receive duplicated block", common.BytesToHexString(hash.ToArrayReverse()))
		return nil
	}
	if err := ledger.DefaultLedger.Blockchain.AddBlock(&msg.blk); err != nil {
		log.Warning("Block add failed: ", err, " ,block hash is ", hash)
		return err
	}
	node.RemoveFlightHeight(msg.blk.Header.Height)
	return nil
}

func (msg dataReq) Handle(node *RemoteNode) error {
	reqtype := common.InventoryType(msg.dataType)
	hash := msg.hash
	switch reqtype {
	case common.BLOCK:
		block, err := NewBlockFromHash(hash)
		if err != nil {
			b, err := NewNotFound(hash)
			node.SendBytesAsync(b)
			return err
		}
		buf, err := NewBlock(block)
		if err != nil {
			return err
		}
		node.SendBytesAsync(buf)

	case common.TRANSACTION:
		txn, err := NewTxnFromHash(hash)
		if err != nil {
			return err
		}
		buf, err := NewTxn(txn)
		if err != nil {
			return err
		}
		node.SendBytesAsync(buf)
	}
	return nil
}

func NewBlockFromHash(hash common.Uint256) (*ledger.Block, error) {
	bk, err := ledger.DefaultLedger.Store.GetBlock(hash)
	if err != nil {
		return nil, err
	}
	return bk, nil
}

func NewBlock(bk *ledger.Block) ([]byte, error) {
	var msg block
	msg.blk = *bk
	msg.msgHdr.Magic = NetID
	cmd := "block"
	copy(msg.msgHdr.CMD[0:len(cmd)], cmd)
	tmpBuffer := bytes.NewBuffer([]byte{})
	bk.Serialize(tmpBuffer)
	p := new(bytes.Buffer)
	err := binary.Write(p, binary.LittleEndian, tmpBuffer.Bytes())
	if err != nil {
		log.Error("Binary Write failed at new Msg")
		return nil, err
	}
	s := sha256.Sum256(p.Bytes())
	s2 := s[:]
	s = sha256.Sum256(s2)
	buf := bytes.NewBuffer(s[:4])
	binary.Read(buf, binary.LittleEndian, &(msg.msgHdr.Checksum))
	msg.msgHdr.Length = uint32(len(p.Bytes()))

	msgBuff := bytes.NewBuffer(nil)
	err = msg.Serialize(msgBuff)
	if err != nil {
		log.Error("new block message error")
		return nil, err
	}

	return msgBuff.Bytes(), nil
}

func ReqBlkData(node *RemoteNode, hash common.Uint256) error {
	var msg dataReq
	msg.dataType = common.BLOCK
	msg.hash = hash

	msg.msgHdr.Magic = NetID
	copy(msg.msgHdr.CMD[0:7], "getdata")
	p := bytes.NewBuffer([]byte{})
	err := binary.Write(p, binary.LittleEndian, &(msg.dataType))
	msg.hash.Serialize(p)
	if err != nil {
		log.Error("Binary Write failed at new getdata Msg")
		return err
	}
	s := sha256.Sum256(p.Bytes())
	s2 := s[:]
	s = sha256.Sum256(s2)
	buf := bytes.NewBuffer(s[:4])
	binary.Read(buf, binary.LittleEndian, &(msg.msgHdr.Checksum))
	msg.msgHdr.Length = uint32(len(p.Bytes()))

	sendBuf := bytes.NewBuffer(nil)
	err = msg.Serialize(sendBuf)
	if err != nil {
		return err
	}
	node.SendBytesAsync(sendBuf.Bytes())

	return nil
}

func (msg block) Verify(buf []byte) error {
	err := msg.msgHdr.Verify(buf)
	// TODO verify the message Content
	return err
}

func (msg block) Serialize(w io.Writer) error {
	err := msg.msgHdr.Serialize(w)
	if err != nil {
		return err
	}
	err = msg.blk.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (msg *block) Deserialize(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &(msg.msgHdr))
	if err != nil {
		log.Error("block header message parsing error")
		return err
	}
	err = msg.blk.Deserialize(r)
	if err != nil {
		log.Error("block message parsing error")
		return err
	}

	return nil
}

type headersReq struct {
	hdr msgHdr
	p   struct {
		len       uint8
		hashStart [HashLen]byte
		hashEnd   [HashLen]byte
	}
}

type blkHeader struct {
	hdr    msgHdr
	cnt    uint32
	blkHdr []ledger.Header
}

func NewHeadersReq(stopHash common.Uint256) ([]byte, error) {
	var h headersReq

	h.p.len = 1
	buf := ledger.DefaultLedger.Store.GetCurrentHeaderHash()
	copy(h.p.hashStart[:], buf[:])
	copy(h.p.hashEnd[:], stopHash[:])

	p := new(bytes.Buffer)
	err := binary.Write(p, binary.LittleEndian, &(h.p))
	if err != nil {
		log.Error("Binary Write failed at new headersReq")
		return nil, err
	}

	s := checkSum(p.Bytes())
	h.hdr.init("getheaders", s, uint32(len(p.Bytes())))

	reqBuff := bytes.NewBuffer(nil)
	err = h.Serialize(reqBuff)
	if err != nil {
		return nil, err
	}

	return reqBuff.Bytes(), err
}

func (msg headersReq) Verify(buf []byte) error {
	return msg.hdr.Verify(buf)
}

func (msg blkHeader) Verify(buf []byte) error {
	return msg.hdr.Verify(buf)
}

func (msg headersReq) Serialize(w io.Writer) error {
	err := msg.hdr.Serialize(w)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, msg.p.len)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, msg.p.hashStart)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, msg.p.hashEnd)
	if err != nil {
		return err
	}

	return nil
}

func (msg *headersReq) Deserialize(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &(msg.hdr))
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &(msg.p.len))
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &(msg.p.hashStart))
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &(msg.p.hashEnd))
	if err != nil {
		return err
	}

	return nil
}

func (msg blkHeader) Serialize(w io.Writer) error {
	err := msg.hdr.Serialize(w)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, msg.cnt)
	if err != nil {
		return err
	}
	for _, header := range msg.blkHdr {
		err = header.Serialize(w)
		if err != nil {
			return err
		}
	}

	return nil
}

func (msg *blkHeader) Deserialize(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &(msg.hdr))
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &(msg.cnt))
	if err != nil {
		return err
	}
	for i := 0; i < int(msg.cnt); i++ {
		var headers ledger.Header
		err := (&headers).Deserialize(r)
		msg.blkHdr = append(msg.blkHdr, headers)
		if err != nil {
			return err
		}
	}

	return nil
}

func (msg headersReq) Handle(node *RemoteNode) error {
	var startHash [HashLen]byte
	var stopHash [HashLen]byte
	startHash = msg.p.hashStart
	stopHash = msg.p.hashEnd
	//FIXME if HeaderHashCount > 1
	headers, cnt, err := GetHeadersFromHash(startHash, stopHash)
	if err != nil {
		return err
	}
	buf, err := NewHeaders(headers, cnt)
	if err != nil {
		return err
	}
	node.SendBytesAsync(buf)
	return nil
}

func SendMsgSyncHeaders(node *RemoteNode, stopHash common.Uint256) {
	buf, err := NewHeadersReq(stopHash)
	if err != nil {
		log.Error("failed build a new headersReq")
	} else {
		node.SendBytesAsync(buf)
	}
}

func (msg blkHeader) Handle(node *RemoteNode) error {
	err := ledger.DefaultLedger.Store.AddHeaders(msg.blkHdr, ledger.DefaultLedger)
	if err != nil {
		log.Warning("Add block Header error")
		return errors.New("Add block Header error, send new header request to another node\n")
	}
	return nil
}

func GetHeadersFromHash(startHash common.Uint256, stopHash common.Uint256) ([]ledger.Header, uint32, error) {
	var count uint32 = 0
	headers := []ledger.Header{}
	var startHeight uint32
	var stopHeight uint32
	if startHash == common.EmptyUint256 {
		return nil, 0, errors.New("invalid start hash for getting headers")
	}
	// get start height
	bkstart, err := ledger.DefaultLedger.Store.GetHeader(startHash)
	if err != nil {
		return nil, 0, err
	}
	startHeight = bkstart.Height

	// get stop height
	if stopHash == common.EmptyUint256 {
		stopHeight = ledger.DefaultLedger.Store.GetHeaderHeight()
	} else {
		bkstop, err := ledger.DefaultLedger.Store.GetHeader(stopHash)
		if err != nil {
			return nil, 0, err
		}
		stopHeight = bkstop.Height
	}

	if startHeight > stopHeight {
		return nil, 0, errors.New("do not have header to send")
	}

	// get header counts to be sent
	count = stopHeight - startHeight
	if count >= MaxHdrCnt {
		count = MaxHdrCnt
	}

	var i uint32
	for i = 1; i <= count; i++ {
		hash, err := ledger.DefaultLedger.Store.GetBlockHash(startHeight + i)
		if err != nil {
			return nil, 0, err
		}
		hd, err := ledger.DefaultLedger.Store.GetHeader(hash)
		if err != nil {
			return nil, 0, err
		}
		headers = append(headers, *hd)
	}

	return headers, count, nil
}

func NewHeaders(headers []ledger.Header, count uint32) ([]byte, error) {
	var msg blkHeader
	msg.cnt = count
	msg.blkHdr = headers
	msg.hdr.Magic = NetID
	cmd := "headers"
	copy(msg.hdr.CMD[0:len(cmd)], cmd)

	tmpBuffer := bytes.NewBuffer([]byte{})
	serialization.WriteUint32(tmpBuffer, msg.cnt)
	for _, header := range headers {
		header.Serialize(tmpBuffer)
	}
	b := new(bytes.Buffer)
	err := binary.Write(b, binary.LittleEndian, tmpBuffer.Bytes())
	if err != nil {
		log.Error("Binary Write failed at new Msg")
		return nil, err
	}
	s := sha256.Sum256(b.Bytes())
	s2 := s[:]
	s = sha256.Sum256(s2)
	buf := bytes.NewBuffer(s[:4])
	binary.Read(buf, binary.LittleEndian, &(msg.hdr.Checksum))
	msg.hdr.Length = uint32(len(b.Bytes()))

	headerBuff := bytes.NewBuffer(nil)
	err = msg.Serialize(headerBuff)
	if err != nil {
		return nil, err
	}

	return headerBuff.Bytes(), nil
}

type notFound struct {
	msgHdr
	hash common.Uint256
}

func NewNotFound(hash common.Uint256) ([]byte, error) {
	var msg notFound
	msg.hash = hash
	msg.msgHdr.Magic = NetID
	cmd := "notfound"
	copy(msg.msgHdr.CMD[0:len(cmd)], cmd)
	tmpBuffer := bytes.NewBuffer([]byte{})
	msg.hash.Serialize(tmpBuffer)
	p := new(bytes.Buffer)
	err := binary.Write(p, binary.LittleEndian, tmpBuffer.Bytes())
	if err != nil {
		log.Error("Binary Write failed at new notfound Msg")
		return nil, err
	}
	s := sha256.Sum256(p.Bytes())
	s2 := s[:]
	s = sha256.Sum256(s2)
	buf := bytes.NewBuffer(s[:4])
	binary.Read(buf, binary.LittleEndian, &(msg.msgHdr.Checksum))
	msg.msgHdr.Length = uint32(len(p.Bytes()))

	notfoundBuff := bytes.NewBuffer(nil)
	err = msg.Serialize(notfoundBuff)
	if err != nil {
		log.Error("Error Convert net message ", err.Error())
		return nil, err
	}

	return notfoundBuff.Bytes(), nil
}

func (msg notFound) Verify(buf []byte) error {
	return msg.msgHdr.Verify(buf)
}

func (msg notFound) Serialize(w io.Writer) error {
	err := msg.msgHdr.Serialize(w)
	if err != nil {
		return err
	}
	_, err = msg.hash.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (msg *notFound) Deserialize(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &(msg.msgHdr))
	if err != nil {
		log.Warning("Parse notfound message hdr error")
		return errors.New("Parse notfound message hdr error")
	}
	err = msg.hash.Deserialize(r)
	if err != nil {
		log.Warning("Parse notfound message error")
		return errors.New("Parse notfound message error")
	}

	return nil
}

func (msg notFound) Handle(node *RemoteNode) error {
	return nil
}

type RelayPacket struct {
	SrcAddr           string
	DestID            []byte
	Payload           []byte
	SigChain          *por.SigChain
	MaxHoldingSeconds uint32
}

type RelayMessage struct {
	msgHdr
	Packet RelayPacket
}

func NewRelayPacket(srcAddr string, destID, payload []byte, sigChain *por.SigChain, maxHoldingSeconds uint32) (*RelayPacket, error) {
	relayPakcet := &RelayPacket{
		SrcAddr:           srcAddr,
		DestID:            destID,
		Payload:           payload,
		SigChain:          sigChain,
		MaxHoldingSeconds: maxHoldingSeconds,
	}
	return relayPakcet, nil
}

func NewRelayMessage(packet *RelayPacket) (*RelayMessage, error) {
	var msg RelayMessage
	msg.msgHdr.Magic = NetID
	cmd := "relay"
	copy(msg.msgHdr.CMD[0:len(cmd)], cmd)
	tmpBuffer := bytes.NewBuffer(nil)
	packet.Serialize(tmpBuffer)
	msg.Packet = *packet
	b := new(bytes.Buffer)
	err := binary.Write(b, binary.LittleEndian, tmpBuffer.Bytes())
	if err != nil {
		log.Error("Binary Write failed at new Msg")
		return nil, err
	}
	s := sha256.Sum256(b.Bytes())
	s2 := s[:]
	s = sha256.Sum256(s2)
	buf := bytes.NewBuffer(s[:4])
	binary.Read(buf, binary.LittleEndian, &(msg.msgHdr.Checksum))
	msg.msgHdr.Length = uint32(len(b.Bytes()))

	return &msg, nil
}

func (msg RelayMessage) Handle(node *RemoteNode) error {
	node.LocalNode().GetEvent("relay").Notify(events.EventRelayMsgReceived, &msg.Packet)
	return nil
}

func (msg *RelayMessage) Serialize(w io.Writer) error {
	err := msg.msgHdr.Serialize(w)
	if err != nil {
		return err
	}

	err = msg.Packet.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (msg *RelayMessage) Deserialize(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &(msg.msgHdr))
	if err != nil {
		return err
	}

	err = msg.Packet.Deserialize(r)
	if err != nil {
		return err
	}

	return nil
}

func (msg *RelayMessage) ToBytes() ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	err := msg.Serialize(buffer)
	if err != nil {
		log.Error("Error serializing relay message ", err.Error())
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (packet *RelayPacket) Serialize(w io.Writer) error {
	err := serialization.WriteVarString(w, packet.SrcAddr)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, packet.DestID)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, packet.Payload)
	if err != nil {
		return err
	}

	buf, err := proto.Marshal(packet.SigChain)
	if err != nil {
		return err
	}
	err = serialization.WriteVarBytes(w, buf)
	if err != nil {
		return err
	}

	err = serialization.WriteUint32(w, packet.MaxHoldingSeconds)
	if err != nil {
		return err
	}

	return nil
}

func (packet *RelayPacket) Deserialize(r io.Reader) error {
	srcAddr, err := serialization.ReadVarString(r)
	if err != nil {
		return err
	}
	packet.SrcAddr = srcAddr

	destID, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	packet.DestID = destID

	payload, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	packet.Payload = payload

	buf, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	packet.SigChain = &por.SigChain{}
	err = proto.Unmarshal(buf, packet.SigChain)
	if err != nil {
		return err
	}

	packet.MaxHoldingSeconds, err = serialization.ReadUint32(r)
	if err != nil {
		return err
	}

	return nil
}

type dataReq struct {
	msgHdr
	dataType common.InventoryType
	hash     common.Uint256
}

// Transaction message
type trn struct {
	msgHdr
	txn transaction.Transaction
}

func (msg trn) Handle(node *RemoteNode) error {
	txn := &msg.txn
	if !node.LocalNode().ExistHash(txn.Hash()) {
		node.LocalNode().IncRxTxnCnt()

		// add transaction to pool when in consensus state
		if node.LocalNode().GetSyncState() == pb.PersistFinished {
			if errCode := node.LocalNode().AppendTxnPool(txn); errCode != nknErrors.ErrNoError {
				return fmt.Errorf("[message] VerifyTransaction failed with %v when AppendTxnPool.", errCode)
			}
		}
	}

	return nil
}

func (msg dataReq) Serialize(w io.Writer) error {
	err := msg.msgHdr.Serialize(w)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, msg.dataType)
	if err != nil {
		return err
	}
	_, err = msg.hash.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (msg *dataReq) Deserialize(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &(msg.msgHdr))
	if err != nil {
		log.Error("datareq header message parsing error")
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &(msg.dataType))
	if err != nil {
		log.Error("datareq datatype message parsing error")
		return err
	}
	err = msg.hash.Deserialize(r)
	if err != nil {
		log.Error("datareq hash message parsing error")
		return err
	}

	return nil
}

func NewTxnFromHash(hash common.Uint256) (*transaction.Transaction, error) {
	txn, err := ledger.DefaultLedger.GetTransactionWithHash(hash)
	if err != nil {
		log.Error("Get transaction with hash error: ", err.Error())
		return nil, err
	}

	return txn, nil
}
func NewTxn(txn *transaction.Transaction) ([]byte, error) {
	var msg trn
	msg.msgHdr.Magic = NetID
	cmd := "tx"
	copy(msg.msgHdr.CMD[0:len(cmd)], cmd)
	tmpBuffer := bytes.NewBuffer([]byte{})
	txn.Serialize(tmpBuffer)
	msg.txn = *txn
	b := new(bytes.Buffer)
	err := binary.Write(b, binary.LittleEndian, tmpBuffer.Bytes())
	if err != nil {
		log.Error("Binary Write failed at new Msg")
		return nil, err
	}
	s := sha256.Sum256(b.Bytes())
	s2 := s[:]
	s = sha256.Sum256(s2)
	buf := bytes.NewBuffer(s[:4])
	binary.Read(buf, binary.LittleEndian, &(msg.msgHdr.Checksum))
	msg.msgHdr.Length = uint32(len(b.Bytes()))

	txnBuff := bytes.NewBuffer(nil)
	err = msg.Serialize(txnBuff)
	if err != nil {
		log.Error("Error Convert net message ", err.Error())
		return nil, err
	}

	return txnBuff.Bytes(), nil
}

func (msg trn) Serialize(w io.Writer) error {
	err := msg.msgHdr.Serialize(w)
	if err != nil {
		return err
	}
	err = msg.txn.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (msg *trn) Deserialize(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &(msg.msgHdr))
	if err != nil {
		return err
	}
	err = msg.txn.Deserialize(r)
	if err != nil {
		return err
	}

	return nil
}
