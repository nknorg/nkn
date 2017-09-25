package states

import (
	"io"
	"DNA/common/serialization"
	"DNA/errors"
)

type StateBase struct {
	StateVersion byte
}

func(stateBase *StateBase)Serialize(w io.Writer) error {
	serialization.WriteVarBytes(w, []byte{stateBase.StateVersion})
	return nil
}

func(stateBase *StateBase)Deserialize(r io.Reader) error {
	stateVersion, err := serialization.ReadVarBytes(r)
	if err != nil {
		return errors.NewDetailErr(err, errors.ErrNoCode, "StateBase StateVersion Deserialize fail.")
	}
	stateBase.StateVersion = stateVersion[0]
	return nil
}

