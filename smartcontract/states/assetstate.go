package states

import (
	"nkn-core/common"
	"nkn-core/crypto"
	"io"
	"nkn-core/common/serialization"
	. "nkn-core/errors"
	"bytes"
	"nkn-core/core/asset"
)

type AssetState struct {
	StateBase
	AssetId common.Uint256
	AssetType asset.AssetType
	Name string
	Amount common.Fixed64
	Available common.Fixed64
	Precision byte
	FeeMode byte
	Fee common.Fixed64
	FeeAddress *common.Uint160
	Owner *crypto.PubKey
	Admin common.Uint160
	Issuer common.Uint160
	Expiration uint32
	IsFrozen bool
}

func(assetState *AssetState)Serialize(w io.Writer) error {
	assetState.StateBase.Serialize(w)
	assetState.AssetId.Serialize(w)
	serialization.WriteVarString(w, assetState.Name)
	assetState.Amount.Serialize(w)
	assetState.Available.Serialize(w)
	serialization.WriteVarBytes(w, []byte{assetState.Precision})
	serialization.WriteVarBytes(w, []byte{assetState.FeeMode})
	assetState.Fee.Serialize(w)
	assetState.FeeAddress.Serialize(w)
	assetState.Owner.Serialize(w)
	assetState.Admin.Serialize(w)
	assetState.Issuer.Serialize(w)
	serialization.WriteUint32(w, assetState.Expiration)
	serialization.WriteBool(w, assetState.IsFrozen)
	return nil
}

func(assetState *AssetState)Deserialize(r io.Reader) error {
	u256 := new(common.Uint256)
	u160 := new(common.Uint160)
	f := new(common.Fixed64)
	pubkey := &crypto.PubKey{}
	stateBase := new(StateBase)
	err := stateBase.Deserialize(r)
	if err != nil {
		return err
	}
	assetState.StateBase = *stateBase
	err = u256.Deserialize(r)
	if err != nil{
		return NewDetailErr(err, ErrNoCode, "AssetState AssetId Deserialize fail.")
	}
	assetState.AssetId = *u256
	name, err := serialization.ReadVarString(r)
	if err != nil{
		return NewDetailErr(err, ErrNoCode, "AssetState Name Deserialize fail.")

	}
	assetState.Name = name
	err = f.Deserialize(r)
	if err != nil{
		return NewDetailErr(err, ErrNoCode, "AssetState Amount Deserialize fail.")

	}
	assetState.Amount = *f
	err = f.Deserialize(r)
	if err != nil{
		return NewDetailErr(err, ErrNoCode, "AssetState Available Deserialize fail.")
	}
	assetState.Available = *f
	precisions, err := serialization.ReadVarBytes(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "AssetState Precision Deserialize fail.")
	}
	assetState.Precision = precisions[0]
	feeModes, err := serialization.ReadVarBytes(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "AssetState FeeMode Deserialize fail.")
	}
	assetState.FeeMode = feeModes[0]
	err = f.Deserialize(r)
	if err != nil{
		return NewDetailErr(err, ErrNoCode, "AssetState Fee Deserialize fail.")
	}
	assetState.Fee = *f
	err = u160.Deserialize(r)
	if err != nil{
		return NewDetailErr(err, ErrNoCode, "AssetState FeeAddress Deserialize fail.")
	}
	assetState.FeeAddress = u160
	err = pubkey.DeSerialize(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "AssetState Owner Deserialize fail.")
	}
	assetState.Owner = pubkey
	err = u160.Deserialize(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "AssetState Admin Deserialize fail.")
	}
	assetState.Admin = *u160
	err = u160.Deserialize(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "AssetState Issuer Deserialize fail.")
	}
	assetState.Issuer = *u160
	expiration, err := serialization.ReadUint32(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "AssetState Expiration Deserialize fail.")
	}
	assetState.Expiration = expiration
	isFrozon, err := serialization.ReadBool(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "AssetState IsFrozon Deserialize fail.")
	}
	assetState.IsFrozen = isFrozon
	return nil
}

func(assetState *AssetState) ToArray() []byte {
	b := new(bytes.Buffer)
	assetState.Serialize(b)
	return b.Bytes()
}
