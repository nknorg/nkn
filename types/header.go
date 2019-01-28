package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	. "github.com/nknorg/nkn/errors"
)

type Header struct {
	BlockHeader
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

	pg := *h.Program
	outputHashes, err := ToCodeHash(pg.Code)
	if err != nil {
		return nil, NewDetailErr(err, ErrNoCode, "[Header], GetProgramHashes failed.")
	}
	programHashes = append(programHashes, outputHashes)
	return programHashes, nil
}

func (h *Header) SetPrograms(programs []*Program) {
	if len(programs) != 1 {
		return
	}

	h.Program = programs[0]
}

func (h *Header) GetPrograms() []*Program {
	return []*Program{h.Program}
}

func (h *Header) Hash() Uint256 {
	d := h.GetMessage()
	temp := sha256.Sum256([]byte(d))
	f := sha256.Sum256(temp[:])
	hash := Uint256(f)
	return hash
}

func (h *Header) GetMessage() []byte {
	b_buf := new(bytes.Buffer)
	h.SerializeUnsigned(b_buf)
	return b_buf.Bytes()
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
