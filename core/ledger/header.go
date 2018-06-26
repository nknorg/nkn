package ledger

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/contract/program"
	sig "github.com/nknorg/nkn/core/signature"
	. "github.com/nknorg/nkn/errors"
)

type Header struct {
	Version          uint32
	PrevBlockHash    Uint256
	TransactionsRoot Uint256
	Timestamp        uint32
	Height           uint32
	ConsensusData    uint64
	NextBookKeeper   Uint160
	Program          *program.Program

	hash Uint256
}

//Serialize the blockheader
func (h *Header) Serialize(w io.Writer) error {
	h.SerializeUnsigned(w)
	w.Write([]byte{byte(1)})
	if h.Program != nil {
		h.Program.Serialize(w)
	}
	return nil
}

//Serialize the blockheader data without program
func (h *Header) SerializeUnsigned(w io.Writer) error {
	serialization.WriteUint32(w, h.Version)
	h.PrevBlockHash.Serialize(w)
	h.TransactionsRoot.Serialize(w)
	serialization.WriteUint32(w, h.Timestamp)
	serialization.WriteUint32(w, h.Height)
	serialization.WriteUint64(w, h.ConsensusData)
	h.NextBookKeeper.Serialize(w)
	return nil
}

func (h *Header) Deserialize(r io.Reader) error {
	h.DeserializeUnsigned(r)
	p := make([]byte, 1)
	n, err := r.Read(p)
	if n > 0 {
		x := []byte(p[:])

		if x[0] != byte(1) {
			return NewDetailErr(errors.New("Header Deserialize get format error."), ErrNoCode, "")
		}
	} else {
		return NewDetailErr(errors.New("Header Deserialize get format error."), ErrNoCode, "")
	}

	pg := new(program.Program)
	err = pg.Deserialize(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Header item Program Deserialize failed.")
	}
	h.Program = pg
	return nil
}

func (h *Header) DeserializeUnsigned(r io.Reader) error {
	//Version
	temp, err := serialization.ReadUint32(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Header item Version Deserialize failed.")
	}
	h.Version = temp

	//PrevBlockHash
	preBlock := new(Uint256)
	err = preBlock.Deserialize(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Header item preBlock Deserialize failed.")
	}
	h.PrevBlockHash = *preBlock

	//TransactionsRoot
	txRoot := new(Uint256)
	err = txRoot.Deserialize(r)
	if err != nil {
		return err
	}
	h.TransactionsRoot = *txRoot

	//Timestamp
	temp, _ = serialization.ReadUint32(r)
	h.Timestamp = temp

	//Height
	temp, _ = serialization.ReadUint32(r)
	h.Height = temp

	//consensusData
	h.ConsensusData, _ = serialization.ReadUint64(r)

	//NextBookKeeper
	h.NextBookKeeper.Deserialize(r)

	return nil
}

func (h *Header) GetProgramHashes() ([]Uint160, error) {
	programHashes := []Uint160{}
	zero := Uint256{}

	if h.PrevBlockHash == zero {
		pg := *h.Program
		outputHashes, err := ToCodeHash(pg.Code)
		if err != nil {
			return nil, NewDetailErr(err, ErrNoCode, "[Header], GetProgramHashes failed.")
		}
		programHashes = append(programHashes, outputHashes)
		return programHashes, nil
	} else {
		prev_header, err := DefaultLedger.Store.GetHeader(h.PrevBlockHash)
		if err != nil {
			return programHashes, err
		}
		programHashes = append(programHashes, prev_header.NextBookKeeper)
		return programHashes, nil
	}

}

func (h *Header) SetPrograms(programs []*program.Program) {
	if len(programs) != 1 {
		return
	}
	h.Program = programs[0]
}

func (h *Header) GetPrograms() []*program.Program {
	return []*program.Program{h.Program}
}

func (h *Header) Hash() Uint256 {

	d := sig.GetHashData(h)
	temp := sha256.Sum256([]byte(d))
	f := sha256.Sum256(temp[:])
	hash := Uint256(f)
	return hash
}

func (h *Header) GetMessage() []byte {
	return sig.GetHashData(h)
}

func (h *Header) ToArray() []byte {
	b := new(bytes.Buffer)
	h.Serialize(b)
	return b.Bytes()
}

func (h *Header) MarshalJson() ([]byte, error) {
	headerInfo := &HeaderInfo{
		Version:          h.Version,
		PrevBlockHash:    BytesToHexString(h.PrevBlockHash.ToArrayReverse()),
		TransactionsRoot: BytesToHexString(h.TransactionsRoot.ToArrayReverse()),
		Timestamp:        h.Timestamp,
		Height:           h.Height,
		ConsensusData:    h.ConsensusData,
		NextBookKeeper:   BytesToHexString(h.NextBookKeeper.ToArrayReverse()),
		Hash:             BytesToHexString(h.hash.ToArrayReverse()),
	}

	info, err := h.Program.MarshalJson()
	if err != nil {
		return nil, err
	}
	json.Unmarshal(info, &headerInfo.Program)

	data, err := json.Marshal(headerInfo)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (h *Header) UnmarshalJson(data []byte) error {
	headerInfo := new(HeaderInfo)
	var err error
	if err = json.Unmarshal(data, &headerInfo); err != nil {
		return err
	}

	h.Version = headerInfo.Version
	h.Timestamp = headerInfo.Timestamp
	h.Height = headerInfo.Height
	h.ConsensusData = headerInfo.ConsensusData

	prevHash, err := HexStringToBytesReverse(headerInfo.PrevBlockHash)
	if err != nil {
		return err
	}
	h.PrevBlockHash, err = Uint256ParseFromBytes(prevHash)
	if err != nil {
		return err
	}

	root, err := HexStringToBytesReverse(headerInfo.TransactionsRoot)
	if err != nil {
		return err
	}
	h.TransactionsRoot, err = Uint256ParseFromBytes(root)
	if err != nil {
		return err
	}

	nextBookKeeper, err := HexStringToBytesReverse(headerInfo.NextBookKeeper)
	if err != nil {
		return err
	}
	h.NextBookKeeper, err = Uint160ParseFromBytes(nextBookKeeper)
	if err != nil {
		return err
	}

	info, err := json.Marshal(headerInfo.Program)
	if err != nil {
		return err
	}
	var pg program.Program
	err = pg.UnmarshalJson(info)
	if err != nil {
		return err
	}
	h.Program = &pg

	hash, err := HexStringToBytesReverse(headerInfo.Hash)
	if err != nil {
		return err
	}
	h.hash, err = Uint256ParseFromBytes(hash)
	if err != nil {
		return err
	}

	return nil
}
