package ising

import (
	"io"

	"github.com/nknorg/nkn/common/serialization"
)

type Ping struct {
	height uint32 // which height want to ping
}

func NewPing(height uint32) *Ping {
	return &Ping{
		height: height,
	}
}

func (p *Ping) Serialize(w io.Writer) error {
	err := serialization.WriteUint32(w, p.height)
	if err != nil {
		return err
	}

	return nil
}

func (p *Ping) Deserialize(r io.Reader) error {
	height, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	p.height = height

	return nil
}
