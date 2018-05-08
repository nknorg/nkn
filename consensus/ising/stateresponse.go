package ising

import (
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type StateResponse struct {
	PersistedBlocks map[uint32]Uint256
}

func (sr *StateResponse) Serialize(w io.Writer) error {
	var err error
	cap := len(sr.PersistedBlocks)
	err = serialization.WriteUint32(w, uint32(cap))
	if err != nil {
		return err
	}
	for height, hash := range sr.PersistedBlocks {
		err = serialization.WriteUint32(w, height)
		if err != nil {
			return err
		}
		_, err = hash.Serialize(w)
		if err != nil {
			return err
		}
	}

	return nil
}

func (sr *StateResponse) Deserialize(r io.Reader) error {
	var err error
	cap, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	sr.PersistedBlocks = make(map[uint32]Uint256, cap)
	for i := 0; i < int(cap); i++ {
		height, err := serialization.ReadUint32(r)
		if err != nil {
			return err
		}
		var hash Uint256
		err = hash.Deserialize(r)
		if err != nil {
			return err
		}
		sr.PersistedBlocks[height] = hash
	}

	return nil
}
