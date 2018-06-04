package payload

import (
	"encoding/json"
	"errors"
	"io"

	. "github.com/nknorg/nkn/common"
)

type Withdraw struct {
	ProgramHash Uint160
}

func (p *Withdraw) Data(version byte) []byte {
	return []byte{0}
}

func (p *Withdraw) Serialize(w io.Writer, version byte) error {
	p.ProgramHash.Serialize(w)

	return nil
}

func (p *Withdraw) Deserialize(r io.Reader, version byte) error {
	var err error
	p.ProgramHash = *new(Uint160)
	err = p.ProgramHash.Deserialize(r)
	if err != nil {
		return errors.New("programhash deserialization error")
	}

	return nil
}

func (p *Withdraw) Equal(p2 *Withdraw) bool {
	if p.ProgramHash.CompareTo(p2.ProgramHash) != 0 {
		return false
	}

	return true
}

func (p *Withdraw) MarshalJson() ([]byte, error) {
	wd := &WithdrawInfo{
		ProgramHash: BytesToHexString(p.ProgramHash.ToArray()),
	}

	data, err := json.Marshal(wd)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (p *Withdraw) UnmarshalJson(data []byte) error {
	wd := new(WithdrawInfo)
	var err error
	if err = json.Unmarshal(data, &wd); err != nil {
		return err
	}

	hash, err := HexStringToBytes(wd.ProgramHash)
	if err != nil {
		return err
	}
	p.ProgramHash, err = Uint160ParseFromBytes(hash)
	if err != nil {
		return err
	}

	return nil
}
