package payload

import (
	"encoding/json"
	"io"

	"github.com/nknorg/nkn/common/serialization"
)

const BookKeepingPayloadVersion byte = 0x03
const BookKeepingPayloadVersionBase byte = 0x02

type BookKeeping struct {
	Nonce uint64 `json:"nonce"`
}

func (a *BookKeeping) Data(version byte) []byte {
	return []byte{0}
}

func (a *BookKeeping) Serialize(w io.Writer, version byte) error {
	if version == BookKeepingPayloadVersionBase {
		return nil
	}
	err := serialization.WriteUint64(w, a.Nonce)
	if err != nil {
		return err
	}
	return nil
}

func (a *BookKeeping) Deserialize(r io.Reader, version byte) error {
	if version == BookKeepingPayloadVersionBase {
		return nil
	}
	var err error
	a.Nonce, err = serialization.ReadUint64(r)
	if err != nil {
		return err
	}
	return nil
}
func (a *BookKeeping) Equal(b *BookKeeping) bool {
	if a.Nonce != b.Nonce {
		return false
	}

	return true
}

func (a *BookKeeping) MarshalJson() ([]byte, error) {
	data, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (a *BookKeeping) UnmarshalJson(data []byte) error {
	if err := json.Unmarshal(data, a); err != nil {
		return err
	}

	return nil
}
