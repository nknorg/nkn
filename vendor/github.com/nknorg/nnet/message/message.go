package message

import "github.com/nknorg/nnet/util"

const (
	// MsgIDBytes is the length of message id in RandBytes
	MsgIDBytes = 8
)

// GenID generates a random message id
func GenID() ([]byte, error) {
	id, err := util.RandBytes(MsgIDBytes)
	if err != nil {
		return nil, err
	}
	return id, nil
}
