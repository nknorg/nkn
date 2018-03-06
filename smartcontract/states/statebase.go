package states

import (
	"io"
	"nkn-core/common/serialization"
	"nkn-core/errors"
)

type StateBase struct {
	StateVersion byte
}

func(stateBase *StateBase)Serialize(w io.Writer) error {
	serialization.WriteByte(w, stateBase.StateVersion)
	return nil
}

func(stateBase *StateBase)Deserialize(r io.Reader) error {
	stateVersion, err := serialization.ReadByte(r)
	if err != nil {
		return errors.NewDetailErr(err, errors.ErrNoCode, "StateBase StateVersion Deserialize fail.")
	}
	stateBase.StateVersion = stateVersion
	return nil
}

