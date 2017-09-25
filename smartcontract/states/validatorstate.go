package states

import (
	"DNA/crypto"
	"io"
	"bytes"
)

type ValidatorState struct {
	PublicKey *crypto.PubKey
}

func(v *ValidatorState) Serialize(w io.Writer) error {
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
	b := new(bytes.Buffer)
	v.Serialize(b)
	return b.Bytes()
}