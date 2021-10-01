package block

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/nknorg/nkn/v2/crypto"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/common/serialization"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/program"
	"github.com/nknorg/nkn/v2/signature"
)

type Header struct {
	*pb.Header
	hash                *common.Uint256
	isSignatureVerified bool
	IsHeaderChecked     bool
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
	err := serialization.WriteUint32(w, h.UnsignedHeader.Version)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, h.UnsignedHeader.PrevBlockHash)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, h.UnsignedHeader.TransactionsRoot)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, h.UnsignedHeader.StateRoot)
	if err != nil {
		return err
	}

	err = serialization.WriteUint64(w, uint64(h.UnsignedHeader.Timestamp))
	if err != nil {
		return err
	}

	err = serialization.WriteUint32(w, h.UnsignedHeader.Height)
	if err != nil {
		return err
	}

	err = serialization.WriteUint32(w, uint32(h.UnsignedHeader.WinnerType))
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, h.UnsignedHeader.SignerPk)
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(w, h.UnsignedHeader.SignerId)
	if err != nil {
		return err
	}

	return nil
}

func (h *Header) DeserializeUnsigned(r io.Reader) error {
	var err error

	h.UnsignedHeader.Version, err = serialization.ReadUint32(r)
	if err != nil {
		return err
	}

	h.UnsignedHeader.PrevBlockHash, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	h.UnsignedHeader.TransactionsRoot, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	h.UnsignedHeader.StateRoot, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	timestamp, err := serialization.ReadUint64(r)
	if err != nil {
		return err
	}

	h.UnsignedHeader.Timestamp = int64(timestamp)

	h.UnsignedHeader.Height, err = serialization.ReadUint32(r)
	if err != nil {
		return err
	}

	winnerType, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}

	h.UnsignedHeader.WinnerType = pb.WinnerType(winnerType)

	h.UnsignedHeader.SignerPk, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	h.UnsignedHeader.SignerId, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	return nil
}

func (h *Header) GetSigner() ([]byte, []byte, error) {
	return h.UnsignedHeader.SignerPk, h.UnsignedHeader.SignerId, nil
}

func (h *Header) GetProgramHashes() ([]common.Uint160, error) {
	programHashes := []common.Uint160{}

	programHash, err := program.CreateProgramHash(h.UnsignedHeader.SignerPk)
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

func (h *Header) Hash() common.Uint256 {
	if h.hash != nil {
		return *h.hash
	}
	d := signature.GetHashData(h)
	temp := sha256.Sum256([]byte(d))
	f := sha256.Sum256(temp[:])
	hash := common.Uint256(f)
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
		PrevBlockHash:    hex.EncodeToString(h.UnsignedHeader.PrevBlockHash),
		TransactionsRoot: hex.EncodeToString(h.UnsignedHeader.TransactionsRoot),
		StateRoot:        hex.EncodeToString(h.UnsignedHeader.StateRoot),
		Timestamp:        h.UnsignedHeader.Timestamp,
		Height:           h.UnsignedHeader.Height,
		RandomBeacon:     hex.EncodeToString(h.UnsignedHeader.RandomBeacon),
		WinnerHash:       hex.EncodeToString(h.UnsignedHeader.WinnerHash),
		WinnerType:       h.UnsignedHeader.WinnerType.String(),
		SignerPk:         hex.EncodeToString(h.UnsignedHeader.SignerPk),
		SignerId:         hex.EncodeToString(h.UnsignedHeader.SignerId),
		Signature:        hex.EncodeToString(h.Signature),
		Hash:             hash.ToHexString(),
	}

	marshaledInfo, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}
	return marshaledInfo, nil
}

func (h *Header) VerifySignature() error {
	if h.isSignatureVerified {
		return nil
	}
	err := crypto.Verify(h.UnsignedHeader.SignerPk, signature.GetHashForSigning(h), h.Signature)
	if err != nil {
		return err
	}
	h.isSignatureVerified = true

	return nil
}
