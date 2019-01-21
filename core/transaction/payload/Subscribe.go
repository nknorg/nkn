package payload

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/util/address"
)

type Subscribe struct {
	Subscriber []byte
	Identifier string
	Topic      string
	Bucket     uint32
	Duration   uint32
	Meta       string
}

func (s *Subscribe) Data(version byte) []byte {
	//TODO: implement Subscribe.Data()
	return []byte{0}

}

func (s *Subscribe) Serialize(w io.Writer, version byte) error {
	serialization.WriteVarBytes(w, s.Subscriber)
	serialization.WriteVarString(w, s.Identifier)
	serialization.WriteVarString(w, s.Topic)
	if version >= 1 {
		serialization.WriteUint32(w, s.Bucket)
	}
	serialization.WriteUint32(w, s.Duration)
	if version >= 2 {
		serialization.WriteVarString(w, s.Meta)
	}
	return nil
}

func (s *Subscribe) Deserialize(r io.Reader, version byte) error {
	var err error
	s.Subscriber, err = serialization.ReadVarBytes(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "[Subscribe], Subscriber Deserialize failed.")
	}
	s.Identifier, err = serialization.ReadVarString(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "[Subscribe], Identifier Deserialize failed.")
	}
	s.Topic, err = serialization.ReadVarString(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "[Subscribe], Topic Deserialize failed.")
	}
	if version >= 1 {
		s.Bucket, err = serialization.ReadUint32(r)
		if err != nil {
			return NewDetailErr(err, ErrNoCode, "[Subscribe], Bucket Deserialize failed.")
		}
	}
	s.Duration, err = serialization.ReadUint32(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "[Subscribe], Duration Deserialize failed.")
	}
	if version >= 2 {
		s.Meta, err = serialization.ReadVarString(r)
		if err != nil {
			return NewDetailErr(err, ErrNoCode, "[Subscribe], Meta Deserialize failed.")
		}
	}
	return nil
}

func (s *Subscribe) Equal(s2 *Subscribe) bool {
	if !bytes.Equal(s.Subscriber, s2.Subscriber) {
		return false
	}

	if s.Identifier != s2.Identifier {
		return false
	}

	if s.Topic != s2.Topic {
		return false
	}

	if s.Bucket != s2.Bucket {
		return false
	}

	if s.Duration != s2.Duration {
		return false
	}

	if s.Meta != s2.Meta {
		return false
	}

	return true
}

func (s *Subscribe) SubscriberString() string {
	return address.MakeAddressString(s.Subscriber, s.Identifier)
}

func (s *Subscribe) MarshalJson() ([]byte, error) {
	si := &SubscribeInfo{
		Subscriber: common.BytesToHexString(s.Subscriber),
		Identifier: s.Identifier,
		Topic:      s.Topic,
		Bucket:     s.Bucket,
		Duration:   s.Duration,
		Meta:       s.Meta,
	}

	data, err := json.Marshal(si)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Subscribe) UnmarshalJson(data []byte) error {
	si := new(SubscribeInfo)
	var err error
	if err = json.Unmarshal(data, &si); err != nil {
		return err
	}

	s.Subscriber, _ = common.HexStringToBytes(si.Subscriber)

	s.Identifier = si.Identifier

	s.Topic = si.Topic

	s.Bucket = si.Bucket

	s.Duration = si.Duration

	s.Meta = si.Meta

	return nil
}
