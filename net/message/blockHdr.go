package message

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/ledger"
	. "github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/log"
)

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

func NewHeadersReq(stopHash Uint256) ([]byte, error) {
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

func (msg headersReq) Handle(node Noder) error {
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
	node.Tx(buf)
	return nil
}

func SendMsgSyncHeaders(node Noder, stopHash Uint256) {
	buf, err := NewHeadersReq(stopHash)
	if err != nil {
		log.Error("failed build a new headersReq")
	} else {
		node.Tx(buf)
	}
}

func (msg blkHeader) Handle(node Noder) error {
	err := ledger.DefaultLedger.Store.AddHeaders(msg.blkHdr, ledger.DefaultLedger)
	if err != nil {
		log.Warning("Add block Header error")
		return errors.New("Add block Header error, send new header request to another node\n")
	}
	return nil
}

func GetHeadersFromHash(startHash Uint256, stopHash Uint256) ([]ledger.Header, uint32, error) {
	var count uint32 = 0
	headers := []ledger.Header{}
	var startHeight uint32
	var stopHeight uint32
	if startHash == EmptyUint256 {
		return nil, 0, errors.New("invalid start hash for getting headers")
	}
	// get start height
	bkstart, err := ledger.DefaultLedger.Store.GetHeader(startHash)
	if err != nil {
		return nil, 0, err
	}
	startHeight = bkstart.Height

	// get stop height
	if stopHash == EmptyUint256 {
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
