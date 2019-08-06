package pb

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

//Serialize the Program
func (p *Program) Serialize(w io.Writer) error {
	err := serialization.WriteVarBytes(w, p.Parameter)
	if err != nil {
		return fmt.Errorf("Execute Program Serialize Code failed:%v", err)
	}
	err = serialization.WriteVarBytes(w, p.Code)
	if err != nil {
		return fmt.Errorf("Execute Program Serialize Parameter failed:%v", err)
	}

	return nil
}

//Deserialize the Program
func (p *Program) Deserialize(w io.Reader) error {
	val, err := serialization.ReadVarBytes(w)
	if err != nil {
		return fmt.Errorf("Execute Program Deserialize Parameter failed:%v", err)
	}
	p.Parameter = val
	p.Code, err = serialization.ReadVarBytes(w)
	if err != nil {
		return fmt.Errorf("Execute Program Deserialize Code failed:%v", err)
	}
	return nil
}

func (p *Program) MarshalJson() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Program) UnmarshalJson(data []byte) error {
	return json.Unmarshal(data, p)
}

func (p *Payload) Serialize(w io.Writer) error {
	err := serialization.WriteUint32(w, uint32(p.Type))
	if err != nil {
		return err
	}
	err = serialization.WriteVarBytes(w, p.Data)
	if err != nil {
		return err
	}

	return nil
}

func (p *Payload) Deserialize(r io.Reader) error {
	typ, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	p.Type = PayloadType(typ)
	dat, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	p.Data = dat

	return nil
}

func (m *Coinbase) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"sender":    common.BytesToUint160(m.Sender),
		"recipient": common.BytesToUint160(m.Recipient),
		"amount":    m.Amount,
	}
}

func (m *TransferAsset) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"sender":    common.BytesToUint160(m.Sender),
		"recipient": common.BytesToUint160(m.Recipient),
		"amount":    m.Amount,
	}
}

func (m *GenerateID) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"publicKey":       common.HexStr(m.PublicKey),
		"registrationFee": m.RegistrationFee,
	}
}

func (m *RegisterName) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"registrant": common.HexStr(m.Registrant),
		"name":       m.Name,
	}
}

func (m *Subscribe) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"subscriber": common.HexStr(m.Subscriber),
		"identifier": m.Identifier,
		"topic":      m.Topic,
		"bucket":     m.Bucket,
		"duration":   m.Duration,
		"meta":       m.Meta,
	}
}

func (m *NanoPay) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"sender":            common.BytesToUint160(m.Sender),
		"recipient":         common.BytesToUint160(m.Recipient),
		"id":                m.Id,
		"amount":            m.Amount,
		"txnExpiration":     m.TxnExpiration,
		"nanoPayExpiration": m.NanoPayExpiration,
	}
}

func (m *SigChainTxn) ToMap() map[string]interface{} {
	sc := &SigChain{}
	if err := sc.Unmarshal(m.SigChain); err != nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"SigChain":  sc.ToMap(),
		"Submitter": common.BytesToUint160(m.Submitter),
	}
}
