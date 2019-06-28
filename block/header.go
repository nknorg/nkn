package block

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gogo/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/program"
	"github.com/nknorg/nkn/signature"
)

type Header struct {
	*pb.Header
	hash *Uint256
}

func (h *Header) Marshal() (buf []byte, err error) {
	return proto.Marshal(h.Header)
}

func (h *Header) Unmarshal(buf []byte) error {
	if h.Header == nil {
		h.Header = &pb.Header{}
	}
	return proto.Unmarshal(buf, h.Header)
}

//Serialize the blockheader data without program
func (h *Header) SerializeUnsigned(w io.Writer) error {
	serialization.WriteUint32(w, h.UnsignedHeader.Version)
	serialization.WriteVarBytes(w, h.UnsignedHeader.PrevBlockHash)
	serialization.WriteVarBytes(w, h.UnsignedHeader.TransactionsRoot)
	serialization.WriteVarBytes(w, h.UnsignedHeader.StateRoot)
	serialization.WriteUint64(w, uint64(h.UnsignedHeader.Timestamp))
	serialization.WriteUint32(w, h.UnsignedHeader.Height)
	serialization.WriteUint32(w, uint32(h.UnsignedHeader.WinnerType))
	serialization.WriteVarBytes(w, h.UnsignedHeader.SignerPk)
	serialization.WriteVarBytes(w, h.UnsignedHeader.SignerId)

	return nil
}

func (h *Header) DeserializeUnsigned(r io.Reader) error {
	h.UnsignedHeader.Version, _ = serialization.ReadUint32(r)
	h.UnsignedHeader.PrevBlockHash, _ = serialization.ReadVarBytes(r)
	h.UnsignedHeader.TransactionsRoot, _ = serialization.ReadVarBytes(r)
	h.UnsignedHeader.StateRoot, _ = serialization.ReadVarBytes(r)
	timestamp, _ := serialization.ReadUint64(r)
	h.UnsignedHeader.Timestamp = int64(timestamp)
	h.UnsignedHeader.Height, _ = serialization.ReadUint32(r)
	winnerType, _ := serialization.ReadUint32(r)
	h.UnsignedHeader.WinnerType = pb.WinnerType(winnerType)
	h.UnsignedHeader.SignerPk, _ = serialization.ReadVarBytes(r)
	h.UnsignedHeader.SignerId, _ = serialization.ReadVarBytes(r)

	return nil
}

func (h *Header) GetProgramHashes() ([]Uint160, error) {
	programHashes := []Uint160{}

	pubKey, err := crypto.NewPubKeyFromBytes(h.UnsignedHeader.SignerPk)
	if err != nil {
		return nil, fmt.Errorf("[Header], Get publick key failed: %v", err)
	}

	programHash, err := program.CreateProgramHash(pubKey)
	if err != nil {
		return nil, fmt.Errorf("[Header], GetProgramHashes failed: %v", err)
	}

	programHashes = append(programHashes, programHash)
	return programHashes, nil
}

func (h *Header) SetPrograms(programs []*pb.Program) {
	return
}

func (h *Header) GetPrograms() []*pb.Program {
	return nil
}

func (h *Header) Hash() Uint256 {
	if h.hash != nil {
		return *h.hash
	}
	d := signature.GetHashData(h)
	temp := sha256.Sum256([]byte(d))
	f := sha256.Sum256(temp[:])
	hash := Uint256(f)
	h.hash = &hash
	return hash
}

func (h *Header) GetMessage() []byte {
	return signature.GetHashData(h)
}

func (h *Header) ToArray() []byte {
	dt, _ := h.Marshal()
	return dt
}

func (h *Header) GetInfo() ([]byte, error) {
	type headerInfo struct {
		Version          uint32 `json:"version"`
		PrevBlockHash    string `json:"prevBlockHash"`
		TransactionsRoot string `json:"transactionsRoot"`
		StateRoot        string `json:"stateRoot"`
		Timestamp        int64  `json:"timestamp"`
		Height           uint32 `json:"height"`
		RandomBeacon     string `json:"randomBeacon"`
		WinnerHash       string `json:"winnerHash"`
		WinnerType       string `json:"winnerType"`
		SignerPk         string `json:"signerPk"`
		SignerId         string `json:"signerId"`
		Signature        string `json:"signature"`
		Hash             string `json:"hash"`
	}

	hash := h.Hash()
	info := &headerInfo{
		Version:          h.UnsignedHeader.Version,
		PrevBlockHash:    BytesToHexString(h.UnsignedHeader.PrevBlockHash),
		TransactionsRoot: BytesToHexString(h.UnsignedHeader.TransactionsRoot),
		StateRoot:        BytesToHexString(h.UnsignedHeader.StateRoot),
		Timestamp:        h.UnsignedHeader.Timestamp,
		Height:           h.UnsignedHeader.Height,
		RandomBeacon:     BytesToHexString(h.UnsignedHeader.RandomBeacon),
		WinnerHash:       BytesToHexString(h.UnsignedHeader.WinnerHash),
		WinnerType:       h.UnsignedHeader.WinnerType.String(),
		SignerPk:         BytesToHexString(h.UnsignedHeader.SignerPk),
		SignerId:         BytesToHexString(h.UnsignedHeader.SignerId),
		Signature:        BytesToHexString(h.Signature),
		Hash:             hash.ToHexString(),
	}

	marshaledInfo, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}
	return marshaledInfo, nil
}
