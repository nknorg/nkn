package payload

import (
	"io"
)

type Coinbase struct {
}

func (c *Coinbase) Data(version byte) []byte {
	return []byte{0}
}

func (c *Coinbase) Serialize(w io.Writer, version byte) error {
	return nil
}

func (c *Coinbase) Deserialize(r io.Reader, version byte) error {
	return nil
}

func (c *Coinbase) MarshalJson() ([]byte, error) {
	return nil, nil
}

func (c *Coinbase) UnmarshalJson(data []byte) error {
	return nil
}
