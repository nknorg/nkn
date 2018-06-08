package asset

import (
	"bytes"
	"errors"
	"io"

	"github.com/nknorg/nkn/common/serialization"
)

type AssetType byte

const (
	Token AssetType = 0x00
)

const (
	MaxPrecision byte = 8
	MinPrecision byte = 0
)

type Asset struct {
	Name        string
	Description string
	Precision   byte
	AssetType   AssetType
}

func (a *Asset) Serialize(w io.Writer) error {
	err := serialization.WriteVarString(w, a.Name)
	if err != nil {
		return errors.New("asset name serialization failed")
	}
	err = serialization.WriteVarString(w, a.Description)
	if err != nil {
		return errors.New("asset description serialization failed")
	}
	err = serialization.WriteByte(w, byte(a.Precision))
	if err != nil {
		return errors.New("asset precision serialization failed")
	}
	err = serialization.WriteByte(w, byte(a.AssetType))
	if err != nil {
		return errors.New("asset type serialization failed")
	}

	return nil
}

func (a *Asset) Deserialize(r io.Reader) error {
	name, err := serialization.ReadVarString(r)
	if err != nil {
		return errors.New("asset name deserialization failed")
	}
	a.Name = name

	description, err := serialization.ReadVarString(r)
	if err != nil {
		return errors.New("asset description deserialization failed")
	}
	a.Description = description

	precision, err := serialization.ReadByte(r)
	if err != nil {
		return errors.New("asset precision deserialization failed")
	}
	a.Precision = precision

	assetType, err := serialization.ReadByte(r)
	if err != nil {
		return errors.New("asset type deserialization failed")
	}
	a.AssetType = AssetType(assetType)

	return nil
}

func (a *Asset) Equal(b *Asset) bool {
	if a.Name != b.Name || a.Description != b.Description ||
		a.Precision != b.Precision || a.AssetType != b.AssetType {
		return false
	}

	return true
}

func (a *Asset) ToArray() []byte {
	b := new(bytes.Buffer)
	a.Serialize(b)
	return b.Bytes()
}
