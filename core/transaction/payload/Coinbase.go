package payload

import (
	"encoding/json"
	"io"

	"github.com/nknorg/nkn/common"
)

type Coinbase struct {
	Sender    common.Uint160
	Recipient common.Uint160
	Amount    common.Fixed64
}

func (c *Coinbase) Data(version byte) []byte {
	return []byte{0}
}

func (c *Coinbase) Serialize(w io.Writer, version byte) error {
	c.Sender.Serialize(w)
	c.Recipient.Serialize(w)
	c.Amount.Serialize(w)

	return nil
}

func (c *Coinbase) Deserialize(r io.Reader, version byte) error {
	c.Sender.Deserialize(r)
	c.Recipient.Deserialize(r)
	c.Amount.Deserialize(r)

	return nil
}

func (c *Coinbase) MarshalJson() ([]byte, error) {
	cb := &CoinbaseInfo{
		Sender:    common.BytesToHexString(c.Sender.ToArray()),
		Recipient: common.BytesToHexString(c.Recipient.ToArray()),
		Amount:    c.Amount.String(),
	}

	data, err := json.Marshal(cb)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (c *Coinbase) UnmarshalJson(data []byte) error {
	cb := new(TransferAssetInfo)

	var err error
	if err = json.Unmarshal(data, &cb); err != nil {
		return err
	}

	sender, err := common.HexStringToBytes(cb.Sender)
	if err != nil {
		return err
	}
	c.Sender, err = common.Uint160ParseFromBytes(sender)
	if err != nil {
		return err
	}

	recipient, err := common.HexStringToBytes(cb.Recipient)
	if err != nil {
		return err
	}
	c.Recipient, err = common.Uint160ParseFromBytes(recipient)
	if err != nil {
		return err
	}

	c.Amount, err = common.StringToFixed64(cb.Amount)
	if err != nil {
		return err
	}

	return nil
}
