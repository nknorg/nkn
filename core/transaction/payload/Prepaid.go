package payload

import (
	. "nkn-core/common"
	"io"
	"errors"
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
