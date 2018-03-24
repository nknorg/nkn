package payload

import (
	"errors"
	"io"
	. "nkn-core/common"
)

type Withdraw struct {
	ProgramHash Uint160
	Amount      Fixed64
}

func (p *Withdraw) Data(version byte) []byte {
	return []byte{0}
}

func (p *Withdraw) Serialize(w io.Writer, version byte) error {
	p.ProgramHash.Serialize(w)
	p.Amount.Serialize(w)

	return nil
}

func (p *Withdraw) Deserialize(r io.Reader, version byte) error {
	var err error
	p.ProgramHash = *new(Uint160)
	err = p.ProgramHash.Deserialize(r)
	if err != nil {
		return errors.New("programhash deserialization error")
	}

	p.Amount = *new(Fixed64)
	err = p.Amount.Deserialize(r)
	if err != nil {
		return errors.New("amount deserialization error")
	}

	return nil
}
