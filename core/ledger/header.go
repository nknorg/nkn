package ledger

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/por"
	sig "github.com/nknorg/nkn/signature"
	"github.com/nknorg/nkn/types"
)

type WinnerType byte

const (
	NumGenesisBlocks = por.SigChainMiningHeightOffset + por.SigChainBlockHeightOffset - 1
	HeaderVersion    = 1
)

type Header struct {
	types.BlockHeader
	hash Uint256
}

//Serialize the blockheader
func (h *Header) Serialize(w io.Writer) error {
	data, err := h.Marshal()
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, data)
	return err
}

//Serialize the blockheader data without program
func (h *Header) SerializeUnsigned(w io.Writer) error {
	data, err := h.UnsignedHeader.Marshal()
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, data)
	return err
}

func (h *Header) Deserialize(r io.Reader) error {
	data, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	err = h.Unmarshal(data)
	return err

}

func (h *Header) DeserializeUnsigned(r io.Reader) error {
	data, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	err = h.UnsignedHeader.Unmarshal(data)
	return err
}

func (h *Header) GetProgramHashes() ([]Uint160, error) {
	programHashes := []Uint160{}
	zero := Uint256{}

	prevHash, err := Uint256ParseFromBytes(h.UnsignedHeader.PrevBlockHash)
	if err != nil {
		return programHashes, err
	}
	if prevHash == zero {
		pg := *h.Program
		outputHashes, err := ToCodeHash(pg.Code)
		if err != nil {
			return nil, NewDetailErr(err, ErrNoCode, "[Header], GetProgramHashes failed.")
		}
		programHashes = append(programHashes, outputHashes)
		return programHashes, nil
	} else {
		prev_header, err := DefaultLedger.Store.GetHeader(prevHash)
		if err != nil {
			return programHashes, err
		}
		programHashes = append(programHashes, BytesToUint160(prev_header.UnsignedHeader.NextBookKeeper))
		return programHashes, nil
	}

}

func (h *Header) SetPrograms(programs []*types.Program) {
	if len(programs) != 1 {
		return
	}

	h.Program = programs[0]
}

func (h *Header) GetPrograms() []*types.Program {
	return []*types.Program{h.Program}
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
	return json.Marshal(h)
}

func (h *Header) UnmarshalJson(data []byte) error {
	return json.Unmarshal(data, h)
}
