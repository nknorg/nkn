package payload

import (
	"io"

	"github.com/nknorg/nkn/common/serialization"
)

type Commit struct {
	SigChain []byte
}

func (p *Commit) Data(version byte) []byte {
	return []byte{0}
}

func (p *Commit) Serialize(w io.Writer, version byte) error {
	err := serialization.WriteVarBytes(w, p.SigChain)
	if err != nil {
		return err
	}

	return nil
}

func (p *Commit) Deserialize(r io.Reader, version byte) error {
	var err error
	p.SigChain, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	return nil
}
