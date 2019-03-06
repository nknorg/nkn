package pb

import (
	"io"

	"github.com/nknorg/nkn/common/serialization"
)

func (p *Payload) Serialize(w io.Writer) error {
	err := serialization.WriteUint32(w, uint32(p.Type))
	if err != nil {
		return err
	}
	err = serialization.WriteVarBytes(w, p.Data)
	if err != nil {
		return err
	}

	return nil
}

func (p *Payload) Deserialize(r io.Reader) error {
	typ, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	p.Type = PayloadType(typ)
	dat, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	p.Data = dat

	return nil
}
