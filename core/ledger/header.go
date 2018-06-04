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
func (bd *Header) Serialize(w io.Writer) error {
	bd.SerializeUnsigned(w)
	w.Write([]byte{byte(1)})
	if bd.Program != nil {
		bd.Program.Serialize(w)
	}
	return nil
}

//Serialize the blockheader data without program
func (bd *Header) SerializeUnsigned(w io.Writer) error {
	//REVD: implement blockheader SerializeUnsigned
	serialization.WriteUint32(w, bd.Version)
	bd.PrevBlockHash.Serialize(w)
	bd.TransactionsRoot.Serialize(w)
	serialization.WriteUint32(w, bd.Timestamp)
	serialization.WriteUint32(w, bd.Height)
	serialization.WriteUint64(w, bd.ConsensusData)
	bd.NextBookKeeper.Serialize(w)
	return nil
}

func (bd *Header) Deserialize(r io.Reader) error {
	//REVD：Header Deserialize
	bd.DeserializeUnsigned(r)
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
	bd.Program = pg
	return nil
}

func (bd *Header) DeserializeUnsigned(r io.Reader) error {
	//Version
	temp, err := serialization.ReadUint32(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Header item Version Deserialize failed.")
	}
	bd.Version = temp

	//PrevBlockHash
	preBlock := new(Uint256)
	err = preBlock.Deserialize(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Header item preBlock Deserialize failed.")
	}
	bd.PrevBlockHash = *preBlock

	//TransactionsRoot
	txRoot := new(Uint256)
	err = txRoot.Deserialize(r)
	if err != nil {
		return err
	}
	bd.TransactionsRoot = *txRoot

	//Timestamp
	temp, _ = serialization.ReadUint32(r)
	bd.Timestamp = uint32(temp)

	//Height
	temp, _ = serialization.ReadUint32(r)
	bd.Height = uint32(temp)

	//consensusData
	bd.ConsensusData, _ = serialization.ReadUint64(r)

	//NextBookKeeper
	bd.NextBookKeeper.Deserialize(r)

	return nil
}

func (bd *Header) GetProgramHashes() ([]Uint160, error) {
	programHashes := []Uint160{}
	zero := Uint256{}

	if bd.PrevBlockHash == zero {
		pg := *bd.Program
		outputHashes, err := ToCodeHash(pg.Code)
		if err != nil {
			return nil, NewDetailErr(err, ErrNoCode, "[Header], GetProgramHashes failed.")
		}
		programHashes = append(programHashes, outputHashes)
		return programHashes, nil
	} else {
		prev_header, err := DefaultLedger.Store.GetHeader(bd.PrevBlockHash)
		if err != nil {
			return programHashes, err
		}
		programHashes = append(programHashes, prev_header.NextBookKeeper)
		return programHashes, nil
	}

}

func (bd *Header) SetPrograms(programs []*program.Program) {
	if len(programs) != 1 {
		return
	}
	bd.Program = programs[0]
}

func (bd *Header) GetPrograms() []*program.Program {
	return []*program.Program{bd.Program}
}

func (bd *Header) Hash() Uint256 {

	d := sig.GetHashData(bd)
	temp := sha256.Sum256([]byte(d))
	f := sha256.Sum256(temp[:])
	hash := Uint256(f)
	return hash
}

func (bd *Header) GetMessage() []byte {
	return sig.GetHashData(bd)
}

func (bd *Header) ToArray() []byte {
	b := new(bytes.Buffer)
	bd.Serialize(b)
	return b.Bytes()
}

func (bd *Header) MarshalJson() ([]byte, error) {
	headerInfo := &HeaderInfo{
		Version:          bd.Version,
		PrevBlockHash:    BytesToHexString(bd.PrevBlockHash.ToArrayReverse()),
		TransactionsRoot: BytesToHexString(bd.TransactionsRoot.ToArrayReverse()),
		Timestamp:        bd.Timestamp,
		Height:           bd.Height,
		ConsensusData:    bd.ConsensusData,
		NextBookKeeper:   BytesToHexString(bd.NextBookKeeper.ToArrayReverse()),
		Hash:             BytesToHexString(bd.hash.ToArrayReverse()),
	}

	info, err := bd.Program.MarshalJson()
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

func (bd *Header) UnmarshalJson(data []byte) error {
	headerInfo := new(HeaderInfo)
	var err error
	if err = json.Unmarshal(data, &headerInfo); err != nil {
		return err
	}

	bd.Version = headerInfo.Version
	bd.Timestamp = headerInfo.Timestamp
	bd.Height = headerInfo.Height
	bd.ConsensusData = headerInfo.ConsensusData

	prevHash, err := HexStringToBytesReverse(headerInfo.PrevBlockHash)
	if err != nil {
		return err
	}
	bd.PrevBlockHash, err = Uint256ParseFromBytes(prevHash)
	if err != nil {
		return err
	}

	root, err := HexStringToBytesReverse(headerInfo.TransactionsRoot)
	if err != nil {
		return err
	}
	bd.TransactionsRoot, err = Uint256ParseFromBytes(root)
	if err != nil {
		return err
	}

	nextBookKeeper, err := HexStringToBytesReverse(headerInfo.NextBookKeeper)
	if err != nil {
		return err
	}
	bd.NextBookKeeper, err = Uint160ParseFromBytes(nextBookKeeper)
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
	bd.Program = &pg

	hash, err := HexStringToBytesReverse(headerInfo.Hash)
	if err != nil {
		return err
	}
	bd.hash, err = Uint256ParseFromBytes(hash)
	if err != nil {
		return err
	}

	return nil
}
