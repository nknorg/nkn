package transaction

import (
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"io"
)

type UTXOUnspent struct {
	Txid  common.Uint256
	Index uint32
	Value common.Fixed64
}

func (uu *UTXOUnspent) Serialize(w io.Writer) error {
	_, err := uu.Txid.Serialize(w)
	if err != nil {
		return err
	}
	err = serialization.WriteUint32(w, uu.Index)
	if err != nil {
		return err
	}
	err = uu.Value.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (uu *UTXOUnspent) Deserialize(r io.Reader) error {
	uu.Txid.Deserialize(r)

	index, err := serialization.ReadUint32(r)
	uu.Index = uint32(index)
	if err != nil {
		return err
	}

	uu.Value.Deserialize(r)

	return nil
}
