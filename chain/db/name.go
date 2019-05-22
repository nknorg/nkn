package db

import (
	"bytes"
	"strings"

	"github.com/nknorg/nkn/common/serialization"
)

func generateRegistrantKey(registrant []byte) []byte {
	registrantKey := bytes.NewBuffer(nil)
	registrantKey.WriteByte(byte(NS_Registrant))
	serialization.WriteVarBytes(registrantKey, registrant)
	return registrantKey.Bytes()
}

func generateNameKey(name string) []byte {
	nameKey := bytes.NewBuffer(nil)
	nameKey.WriteByte(byte(NS_Name))
	serialization.WriteVarString(nameKey, strings.ToLower(name))
	return nameKey.Bytes()
}

func (cs *ChainStore) SaveName(registrant []byte, name string) error {
	registrantKey := generateRegistrantKey(registrant)
	nameKey := generateNameKey(name)

	// PUT VALUE
	w := bytes.NewBuffer(nil)
	serialization.WriteVarString(w, name)
	err := cs.st.BatchPut(registrantKey, w.Bytes())
	if err != nil {
		return err
	}

	w = bytes.NewBuffer(nil)
	serialization.WriteVarBytes(w, registrant)
	err = cs.st.BatchPut(nameKey, w.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) DeleteName(registrant []byte) error {
	name, err := cs.GetName(registrant)
	if err != nil {
		return err
	}

	registrantKey := generateRegistrantKey(registrant)
	nameKey := generateNameKey(*name)

	// DELETE VALUE
	err = cs.st.BatchDelete(registrantKey)
	if err != nil {
		return err
	}
	err = cs.st.BatchDelete(nameKey)
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) GetName(registrant []byte) (*string, error) {
	registrantKey := generateRegistrantKey(registrant)

	data, err := cs.st.Get(registrantKey)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(data)

	name, err := serialization.ReadVarString(r)
	if err != nil {
		return nil, err
	}

	return &name, nil
}

func (cs *ChainStore) GetRegistrant(name string) ([]byte, error) {
	nameKey := generateNameKey(name)

	data, err := cs.st.Get(nameKey)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(data)

	registrant, err := serialization.ReadVarBytes(r)
	if err != nil {
		return nil, err
	}

	return registrant, nil
}
