package payload

import (
	"encoding/json"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/asset"
	"github.com/nknorg/nkn/crypto"
	. "github.com/nknorg/nkn/errors"
)

const RegisterPayloadVersion byte = 0x00

type RegisterAsset struct {
	Asset  *asset.Asset
	Amount common.Fixed64
	//Precision  byte
	Issuer     *crypto.PubKey
	Controller common.Uint160
}

func (a *RegisterAsset) Data(version byte) []byte {
	//TODO: implement RegisterAsset.Data()
	return []byte{0}

}

func (a *RegisterAsset) Serialize(w io.Writer, version byte) error {
	a.Asset.Serialize(w)
	a.Amount.Serialize(w)
	//w.Write([]byte{a.Precision})
	a.Issuer.Serialize(w)
	a.Controller.Serialize(w)
	return nil
}

func (a *RegisterAsset) Deserialize(r io.Reader, version byte) error {

	//asset
	a.Asset = new(asset.Asset)
	err := a.Asset.Deserialize(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "[RegisterAsset], Asset Deserialize failed.")
	}

	//Amount
	a.Amount = *new(common.Fixed64)
	err = a.Amount.Deserialize(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "[RegisterAsset], Ammount Deserialize failed.")
	}

	//Precision  byte 02/10 comment out by wjj
	//p := make([]byte, 1)
	//n, err := r.Read(p)
	//if n > 0 {
	//	a.Precision = p[0]
	//} else {
	//	return NewDetailErr(err, ErrNoCode, "[RegisterAsset], Precision Deserialize failed.")
	//}

	//Issuer     *crypto.PubKey
	a.Issuer = new(crypto.PubKey)
	err = a.Issuer.Deserialize(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "[RegisterAsset], Ammount Deserialize failed.")
	}

	//Controller *common.Uint160
	a.Controller = *new(common.Uint160)
	err = a.Controller.Deserialize(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "[RegisterAsset], Ammount Deserialize failed.")
	}
	return nil
}
func (a *RegisterAsset) Equal(b *RegisterAsset) bool {
	if !a.Asset.Equal(b.Asset) {
		return false
	}
	if a.Amount.GetData() != b.Amount.GetData() {
		return false
	}
	if !crypto.Equal(a.Issuer, b.Issuer) {
		return false
	}

	if a.Controller.CompareTo(b.Controller) != 0 {
		return false
	}

	return true
}

func (a *RegisterAsset) MarshalJson() ([]byte, error) {
	ra := &RegisterAssetInfo{
		Asset:      a.Asset,
		Amount:     a.Amount.String(),
		Issuer:     a.Issuer,
		Controller: common.BytesToHexString(a.Controller.ToArray()),
	}

	data, err := json.Marshal(ra)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (a *RegisterAsset) UnmarshalJson(data []byte) error {
	ra := new(RegisterAssetInfo)
	var err error
	if err = json.Unmarshal(data, &ra); err != nil {
		return err
	}

	a.Asset = ra.Asset
	a.Amount, err = common.StringToFixed64(ra.Amount)
	if err != nil {
		return err
	}
	a.Issuer = ra.Issuer

	controller, err := common.HexStringToBytes(ra.Controller)
	if err != nil {
		return err
	}
	a.Controller, err = common.Uint160ParseFromBytes(controller)
	if err != nil {
		return err
	}

	return nil
}
