package db

import (
	"fmt"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type nanoPay struct {
	balance   common.Fixed64
	expiresAt uint32
}

func (np *nanoPay) Serialize(w io.Writer) error {
	err := np.balance.Serialize(w)
	if err != nil {
		return fmt.Errorf("nanoPay Serialize error: %v", err)
	}

	err = serialization.WriteUint32(w, np.expiresAt)
	if err != nil {
		return fmt.Errorf("nanoPay Serialize error: %v", err)
	}

	return nil
}

func (np *nanoPay) Deserialize(r io.Reader) error {
	err := np.balance.Deserialize(r)
	if err != nil {
		return fmt.Errorf("Deserialize nanoPay error: %v", err)
	}

	np.expiresAt, err = serialization.ReadUint32(r)
	if err != nil {
		return fmt.Errorf("Deserialize nanoPay error: %v", err)
	}

	return nil
}