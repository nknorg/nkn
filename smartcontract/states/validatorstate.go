package states

import (
	"DNA/crypto"
	"io"
	"bytes"
)

type ValidatorState struct {
	StateBase
	PublicKey *crypto.PubKey
}

func(v *ValidatorState) Serialize(w io.Writer) error {
	v.StateBase.Serialize(w)
	v.PublicKey.Serialize(w)
	return nil
}


func(v *ValidatorState)Deserialize(r io.Reader) error {
	
	p := new(crypto.PubKey)
	p.DeSerialize(r)
	v.PublicKey = p
	return nil
}

func(v *ValidatorState) ToArray() []byte {
	stateBase := new(StateBase)
	err := stateBase.Deserialize(r)
	if err != nil {
		return err
	}
	v.StateBase = *stateBase
	p := new(crypto.PubKey)
	err = p.DeSerialize(r)
	if err != nil {
		return err
	}
	v.PublicKey = p
	return nil
}
