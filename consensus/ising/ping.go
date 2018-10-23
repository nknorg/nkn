package ising

import (
	"io"

	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/net/protocol"
)

type Ping struct {
	height    uint32 // which height want to ping
	syncState protocol.SyncState
}

func NewPing(height uint32, syncState protocol.SyncState) *Ping {
	return &Ping{
		height:    height,
		syncState: syncState,
	}
}

func (p *Ping) Serialize(w io.Writer) error {
	err := serialization.WriteUint32(w, p.height)
	if err != nil {
		return err
	}

	err = serialization.WriteByte(w, byte(p.syncState))
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

	state, err := serialization.ReadByte(r)
	if err != nil {
		return err
	}
	p.syncState = protocol.SyncState(state)

	return nil
}
