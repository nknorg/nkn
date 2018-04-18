package message

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/events"
	. "github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/log"
)

type block struct {
	msgHdr
	blk ledger.Block
}

func (msg block) Handle(node Noder) error {
	log.Debug("RX block message")
	hash := msg.blk.Hash()
	if ledger.DefaultLedger.BlockInLedger(hash) {
		ReceiveDuplicateBlockCnt++
		log.Debug("Receive ", ReceiveDuplicateBlockCnt, " duplicated block.")
		return nil
	}
	if err := ledger.DefaultLedger.Blockchain.AddBlock(&msg.blk); err != nil {
		log.Warn("Block add failed: ", err, " ,block hash is ", hash)
		return err
	}
	node.RemoveFlightHeight(msg.blk.Header.Height)
	node.LocalNode().GetEvent("block").Notify(events.EventNewInventory, &msg.blk)
	return nil
}

func (msg dataReq) Handle(node Noder) error {
	log.Debug()
	reqtype := common.InventoryType(msg.dataType)
	hash := msg.hash
	switch reqtype {
	case common.BLOCK:
		block, err := NewBlockFromHash(hash)
		if err != nil {
			log.Debug("Can't get block from hash: ", hash, " ,send not found message")
			b, err := NewNotFound(hash)
			node.Tx(b)
			return err
		}
		log.Debug("block height is ", block.Header.Height, " ,hash is ", hash)
		buf, err := NewBlock(block)
		if err != nil {
			return err
		}
		node.Tx(buf)

	case common.TRANSACTION:
		txn, err := NewTxnFromHash(hash)
		if err != nil {
			return err
		}
		buf, err := NewTxn(txn)
		if err != nil {
			return err
		}
		go node.Tx(buf)
	}
	return nil
}

func NewBlockFromHash(hash common.Uint256) (*ledger.Block, error) {
	bk, err := ledger.DefaultLedger.Store.GetBlock(hash)
	if err != nil {
		log.Errorf("Get Block error: %s, block hash: %x", err.Error(), hash)
		return nil, err
	}
	return bk, nil
}

func NewBlock(bk *ledger.Block) ([]byte, error) {
	log.Debug()
	var msg block
	msg.blk = *bk
	msg.msgHdr.Magic = NETMAGIC
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
	log.Debug("The message payload length is ", msg.msgHdr.Length)

	msgBuff := bytes.NewBuffer(nil)
	err = msg.Serialize(msgBuff)
	if err != nil {
		log.Error("new block message error")
		return nil, err
	}

	return msgBuff.Bytes(), nil
}

func ReqBlkData(node Noder, hash common.Uint256) error {
	var msg dataReq
	msg.dataType = common.BLOCK
	msg.hash = hash

	msg.msgHdr.Magic = NETMAGIC
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
	node.Tx(sendBuf.Bytes())

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
