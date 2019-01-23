package payload

import (
	"encoding/json"
	"io"

	"github.com/nknorg/nkn/common"
)

type TransferAsset struct {
	Sender    common.Uint160
	Recipient common.Uint160
	Amount    common.Fixed64
}

func (a *TransferAsset) Data(version byte) []byte {
	return []byte{0}

}

func (a *TransferAsset) Serialize(w io.Writer, version byte) error {
	a.Sender.Serialize(w)
	a.Recipient.Serialize(w)
	a.Amount.Serialize(w)

	return nil
}

func (a *TransferAsset) Deserialize(r io.Reader, version byte) error {
	a.Sender.Deserialize(r)
	a.Recipient.Deserialize(r)
	a.Amount.Deserialize(r)
	return nil
}

func (a *TransferAsset) MarshalJson() ([]byte, error) {
	cb := &TransferAssetInfo{
		Sender:    common.BytesToHexString(a.Sender.ToArray()),
		Recipient: common.BytesToHexString(a.Recipient.ToArray()),
		Amount:    a.Amount.String(),
	}

	data, err := json.Marshal(cb)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (a *TransferAsset) UnmarshalJson(data []byte) error {
	cb := new(TransferAssetInfo)

	var err error
	if err = json.Unmarshal(data, &cb); err != nil {
		return err
	}

	sender, err := common.HexStringToBytes(cb.Sender)
	if err != nil {
		return err
	}
	a.Sender, err = common.Uint160ParseFromBytes(sender)
	if err != nil {
		return err
	}

	recipient, err := common.HexStringToBytes(cb.Recipient)
	if err != nil {
		return err
	}
	a.Recipient, err = common.Uint160ParseFromBytes(recipient)
	if err != nil {
		return err
	}

	a.Amount, err = common.StringToFixed64(cb.Amount)
	if err != nil {
		return err
	}

	return nil
}
