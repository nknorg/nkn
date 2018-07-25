package message

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/ledger"
	. "github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/log"
)

var LastInvHash Uint256

type blocksReq struct {
	msgHdr
	p struct {
		HeaderHashCount uint8
		hashStart       [HashLen]byte
		hashStop        [HashLen]byte
	}
}

type InvPayload struct {
	InvType InventoryType
	Cnt     uint32
	Blk     []byte
}

type Inv struct {
	Hdr msgHdr
	P   InvPayload
}

func NewBlocksReq(n Noder) ([]byte, error) {
	var h blocksReq
	// Fixme correct with the exactly request length
	h.p.HeaderHashCount = 1
	//Fixme! Should get the remote Node height.
	buf := ledger.DefaultLedger.Blockchain.CurrentBlockHash()

	copy(h.p.hashStart[:], reverse(buf[:]))

	p := new(bytes.Buffer)
	err := binary.Write(p, binary.LittleEndian, &(h.p))
	if err != nil {
		log.Error("Binary Write failed at new blocksReq")
		return nil, err
	}

	s := checkSum(p.Bytes())
	h.msgHdr.init("getblocks", s, uint32(len(p.Bytes())))

	reqBuff := bytes.NewBuffer(nil)
	err = h.Serialize(reqBuff)
	if err != nil {
		return nil, err
	}

	return reqBuff.Bytes(), nil
}

func (msg blocksReq) Verify(buf []byte) error {
	return msg.msgHdr.Verify(buf)
}

func (msg blocksReq) Handle(node Noder) error {
	var starthash Uint256
	var stophash Uint256
	starthash = msg.p.hashStart
	stophash = msg.p.hashStop
	//FIXME if HeaderHashCount > 1
	inv, err := GetInvFromBlockHash(starthash, stophash)
	if err != nil {
		return err
	}
	buf, err := NewInv(inv)
	if err != nil {
		return err
	}
	go node.Tx(buf)
	return nil
}

func (msg blocksReq) Serialize(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, msg)
}

func (msg *blocksReq) Deserialize(r io.Reader) error {
	return binary.Read(r, binary.LittleEndian, &msg)
}

func (msg Inv) Verify(buf []byte) error {
	return msg.Hdr.Verify(buf)
}

func (msg Inv) Handle(node Noder) error {
	var id Uint256
	invType := InventoryType(msg.P.InvType)
	switch invType {
	case TRANSACTION:
		// TODO check the ID queue
		id.Deserialize(bytes.NewReader(msg.P.Blk[:32]))
		if !node.ExistHash(id) {
			reqTxnData(node, id)
		}
	case BLOCK:
		var i uint32
		count := msg.P.Cnt
		for i = 0; i < count; i++ {
			id.Deserialize(bytes.NewReader(msg.P.Blk[HashLen*i:]))
			// TODO check the ID queue
			if !ledger.DefaultLedger.Store.BlockInCache(id) &&
				!ledger.DefaultLedger.BlockInLedger(id) &&
				LastInvHash != id {
				LastInvHash = id
				// send the block request
				log.Infof("inv request block hash: %x", id)
				ReqBlkData(node, id)
			}
		}
	default:
		log.Warn("RX unknown inventory message")
	}
	return nil
}

func (msg Inv) Serialize(w io.Writer) error {
	err := msg.Hdr.Serialize(w)
	if err != nil {
		return err
	}
	err = msg.P.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (msg *Inv) Deserialize(r io.Reader) error {
	err := msg.Hdr.Deserialize(r)
	if err != nil {
		return err
	}
	invType, err := serialization.ReadUint8(r)
	if err != nil {
		return err
	}
	msg.P.InvType = InventoryType(invType)
	msg.P.Cnt, err = serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &(msg.P.Blk))
	if err != nil {
		return err
	}

	return nil
}

func (msg Inv) invType() InventoryType {
	return msg.P.InvType
}

func GetInvFromBlockHash(starthash Uint256, stophash Uint256) (*InvPayload, error) {
	var count uint32 = 0
	var i uint32
	var empty Uint256
	var startheight uint32
	var stopheight uint32
	curHeight := ledger.DefaultLedger.GetLocalBlockChainHeight()
	if starthash == empty {
		if stophash == empty {
			if curHeight > MaxHdrCnt {
				count = MaxHdrCnt
			} else {
				count = curHeight
			}
		} else {
			bkstop, err := ledger.DefaultLedger.Store.GetHeader(stophash)
			if err != nil {
				return nil, err
			}
			stopheight = bkstop.Height
			count = curHeight - stopheight
			if curHeight > MaxInvHdrCnt {
				count = MaxInvHdrCnt
			}
		}
	} else {
		bkstart, err := ledger.DefaultLedger.Store.GetHeader(starthash)
		if err != nil {
			return nil, err
		}
		startheight = bkstart.Height
		if stophash != empty {
			bkstop, err := ledger.DefaultLedger.Store.GetHeader(stophash)
			if err != nil {
				return nil, err
			}
			stopheight = bkstop.Height
			count = startheight - stopheight
			if count >= MaxInvHdrCnt {
				count = MaxInvHdrCnt
				stopheight = startheight + MaxInvHdrCnt
			}
		} else {

			if startheight > MaxInvHdrCnt {
				count = MaxInvHdrCnt
			} else {
				count = startheight
			}
		}
	}
	tmpBuffer := bytes.NewBuffer([]byte{})
	for i = 1; i <= count; i++ {
		//FIXME need add error handle for GetBlockWithHash
		hash, _ := ledger.DefaultLedger.Store.GetBlockHash(stopheight + i)
		hash.Serialize(tmpBuffer)
	}

	return NewInvPayload(BLOCK, count, tmpBuffer.Bytes()), nil
}

func NewInvPayload(invType InventoryType, count uint32, msg []byte) *InvPayload {
	return &InvPayload{
		InvType: invType,
		Cnt:     count,
		Blk:     msg,
	}
}

func NewInv(inv *InvPayload) ([]byte, error) {
	var msg Inv

	msg.P.Blk = inv.Blk
	msg.P.InvType = inv.InvType
	msg.P.Cnt = inv.Cnt
	msg.Hdr.Magic = NetID
	cmd := "inv"
	copy(msg.Hdr.CMD[0:len(cmd)], cmd)
	tmpBuffer := bytes.NewBuffer([]byte{})
	inv.Serialize(tmpBuffer)

	b := new(bytes.Buffer)
	err := binary.Write(b, binary.LittleEndian, tmpBuffer.Bytes())
	if err != nil {
		log.Error("Binary Write failed at new Msg", err.Error())
		return nil, err
	}
	s := sha256.Sum256(b.Bytes())
	s2 := s[:]
	s = sha256.Sum256(s2)
	buf := bytes.NewBuffer(s[:4])
	binary.Read(buf, binary.LittleEndian, &(msg.Hdr.Checksum))
	msg.Hdr.Length = uint32(len(b.Bytes()))

	invBuff := bytes.NewBuffer(nil)
	err = msg.Serialize(invBuff)
	if err != nil {
		log.Error("Error Convert net message ", err.Error())
		return nil, err
	}

	return invBuff.Bytes(), nil
}

func (msg *InvPayload) Serialize(w io.Writer) error {
	err := serialization.WriteUint8(w, uint8(msg.InvType))
	if err != nil {
		return err
	}
	err = serialization.WriteUint32(w, msg.Cnt)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, msg.Blk)
	if err != nil {
		return err
	}

	return nil
}
