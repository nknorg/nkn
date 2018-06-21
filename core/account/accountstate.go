package account

import (
	"bytes"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type AccountState struct {
	ProgramHash common.Uint160
	IsFrozen    bool
	Balances    map[common.Uint256]common.Fixed64
}

func NewAccountState(programHash common.Uint160, balances map[common.Uint256]common.Fixed64) *AccountState {
	var accountState AccountState
	accountState.ProgramHash = programHash
	accountState.Balances = balances
	accountState.IsFrozen = false
	return &accountState
}

func (as *AccountState) Serialize(w io.Writer) error {
	as.ProgramHash.Serialize(w)
	serialization.WriteBool(w, as.IsFrozen)
	serialization.WriteUint64(w, uint64(len(as.Balances)))
	for k, v := range as.Balances {
		k.Serialize(w)
		v.Serialize(w)
	}
	return nil
}

func (as *AccountState) Deserialize(r io.Reader) error {
	as.ProgramHash.Deserialize(r)
	isFrozen, err := serialization.ReadBool(r)
	if err != nil {
		return err
	}
	as.IsFrozen = isFrozen
	l, err := serialization.ReadUint64(r)
	if err != nil {
		return err
	}
	balances := make(map[common.Uint256]common.Fixed64, 0)
	u := new(common.Uint256)
	f := new(common.Fixed64)
	for i := 0; i < int(l); i++ {
		err = u.Deserialize(r)
		if err != nil {
			return err
		}
		err = f.Deserialize(r)
		if err != nil {
			return err
		}
		balances[*u] = *f
	}
	as.Balances = balances
	return nil
}

func (as *AccountState) ToArray() []byte {
	b := new(bytes.Buffer)
	as.Serialize(b)
	return b.Bytes()
}
