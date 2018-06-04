package payload

import (
	"encoding/json"
	"errors"
	"io"

	. "github.com/nknorg/nkn/common"
)

type Prepaid struct {
	Asset  Uint256
	Amount Fixed64
	Rates  Fixed64 // per byte
}

func (p *Prepaid) Data(version byte) []byte {
	return []byte{0}
}

func (p *Prepaid) Serialize(w io.Writer, version byte) error {
	p.Amount.Serialize(w)
	p.Rates.Serialize(w)

	return nil
}

func (p *Prepaid) Deserialize(r io.Reader, version byte) error {
	p.Amount = *new(Fixed64)
	err := p.Amount.Deserialize(r)
	if err != nil {
		return errors.New("amount deserialization error")
	}

	p.Rates = *new(Fixed64)
	err = p.Rates.Deserialize(r)
	if err != nil {
		return errors.New("rates deserialization error")
	}

	return nil
}

func (p *Prepaid) Equal(b *Prepaid) bool {
	if p.Asset.CompareTo(b.Asset) != 0 {
		return false
	}

	if p.Amount.GetData() != b.Amount.GetData() {
		return false
	}

	if p.Rates.GetData() != b.Rates.GetData() {
		return false
	}

	return true
}

func (p *Prepaid) MarshalJson() ([]byte, error) {
	pp := PrepaidInfo{
		Asset:  BytesToHexString(p.Asset.ToArrayReverse()),
		Amount: p.Amount.String(),
		Rates:  p.Rates.String(),
	}

	data, err := json.Marshal(pp)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (p *Prepaid) UnmarshalJson(data []byte) error {
	ppInfo := new(PrepaidInfo)
	var err error
	if err = json.Unmarshal(data, &ppInfo); err != nil {
		return err
	}

	asset, err := HexStringToBytesReverse(ppInfo.Asset)
	if err != nil {
		return err
	}
	p.Asset, err = Uint256ParseFromBytes(asset)
	if err != nil {
		return err
	}

	p.Amount, err = StringToFixed64(ppInfo.Amount)
	if err != nil {
		return err
	}

	p.Rates, err = StringToFixed64(ppInfo.Rates)
	if err != nil {
		return err
	}

	return nil
}
