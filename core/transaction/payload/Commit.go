package payload

import (
	"encoding/json"
	"io"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type Commit struct {
	SigChain  []byte
	Submitter Uint160
}

func (p *Commit) Data(version byte) []byte {
	return []byte{0}
}

func (p *Commit) Serialize(w io.Writer, version byte) error {
	err := serialization.WriteVarBytes(w, p.SigChain)
	if err != nil {
		return err
	}
	_, err = p.Submitter.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (p *Commit) Deserialize(r io.Reader, version byte) error {
	var err error
	p.SigChain, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	err = p.Submitter.Deserialize(r)
	if err != nil {
		return err
	}

	return nil
}

func (p *Commit) Equal(p2 *Commit) bool {
	if !IsEqualBytes(p.SigChain, p2.SigChain) {
		return false
	}
	if p.Submitter.CompareTo(p2.Submitter) != 0 {
		return false
	}

	return true
}

func (p *Commit) MarshalJson() ([]byte, error) {
	cm := &CommitInfo{
		SigChain:  BytesToHexString(p.SigChain),
		Submitter: BytesToHexString(p.Submitter.ToArray()),
	}

	data, err := json.Marshal(cm)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (p *Commit) UnmarshalJson(data []byte) error {
	cmInfo := new(CommitInfo)
	var err error
	if err = json.Unmarshal(data, &cmInfo); err != nil {
		return err
	}

	p.SigChain, err = HexStringToBytes(cmInfo.SigChain)
	if err != nil {
		return nil
	}

	submitter, err := HexStringToBytes(cmInfo.Submitter)
	if err != nil {
		return err
	}
	p.Submitter, err = Uint160ParseFromBytes(submitter)
	if err != nil {
		return err
	}

	return nil
}
