package db

import (
	"bytes"
	"strings"

	"github.com/nknorg/nkn/common/serialization"
)

func getRegistrantId(registrant []byte) string {
	return string(registrant)
}

func getNameId(name string) string {
	return strings.ToLower(name)
}

func (sdb *StateDB) updateName(registrant []byte, name string) error {
	registrantId := getRegistrantId(registrant)
	nameId := getNameId(name)

	w := bytes.NewBuffer(nil)
	serialization.WriteVarString(w, name)
	err := sdb.trie.TryUpdate(append(NamePrefix, nameId...), w.Bytes())
	if err != nil {
		return err
	}

	w = bytes.NewBuffer(nil)
	serialization.WriteVarBytes(w, registrant)
	return sdb.trie.TryUpdate(append(NameRegistrantPrefix, registrantId...), w.Bytes())
}

func (sdb *StateDB) setName(registrant []byte, name string) {
	registrantId := getRegistrantId(registrant)
	nameId := getNameId(name)

	sdb.names[registrantId] = name
	sdb.nameRegistrants[nameId] = registrant
}

func (cs *ChainStore) SetName(registrant []byte, name string) {
	cs.States.setName(registrant, name)
}

func (sdb *StateDB) deleteName(registrantId string, nameId string) error {
	err := sdb.trie.TryDelete(append(NameRegistrantPrefix, registrantId...))
	if err != nil {
		return err
	}

	err = sdb.trie.TryDelete(append(NamePrefix, nameId...))
	if err != nil {
		return err
	}

	delete(sdb.names, registrantId)
	delete(sdb.nameRegistrants, nameId)

	return nil
}

func (sdb *StateDB) deleteNameForRegistrant(registrant []byte) error {
	name, err := sdb.getName(registrant)
	if err != nil {
		return err
	}
	if name == "" {
		return nil
	}

	registrantId := getRegistrantId(registrant)
	nameId := getNameId(name)

	sdb.names[registrantId] = ""
	sdb.nameRegistrants[nameId] = nil

	return nil
}

func (cs *ChainStore) DeleteName(registrant []byte) error {
	return cs.States.deleteNameForRegistrant(registrant)
}

func (sdb *StateDB) getName(registrant []byte) (string, error) {
	registrantId := getRegistrantId(registrant)
	var name string
	var ok bool
	if name, ok = sdb.names[registrantId]; !ok {
		enc, err := sdb.trie.TryGet(append(NameRegistrantPrefix, registrantId...))
		if err != nil {
			return "", err
		}

		if len(enc) > 0 {
			buff := bytes.NewBuffer(enc)
			name, err = serialization.ReadVarString(buff)
			if err != nil {
				return "", err
			}

			nameId := getNameId(name)
			sdb.names[registrantId] = name
			sdb.nameRegistrants[nameId] = registrant
		}
	}

	return name, nil
}

func (sdb *StateDB) getRegistrant(name string) ([]byte, error) {
	nameId := getNameId(name)
	var registrant []byte
	var ok bool
	if registrant, ok = sdb.nameRegistrants[nameId]; !ok {
		enc, err := sdb.trie.TryGet(append(NamePrefix, nameId...))
		if err != nil {
			return nil, err
		}

		if len(enc) > 0 {
			buff := bytes.NewBuffer(enc)
			registrant, err = serialization.ReadVarBytes(buff)
			if err != nil {
				return nil, err
			}

			registrantId := getRegistrantId(registrant)
			sdb.names[registrantId] = name
			sdb.nameRegistrants[nameId] = registrant
		}
	}

	return registrant, nil
}

func (cs *ChainStore) GetName(registrant []byte) (string, error) {
	return cs.States.getName(registrant)
}

func (cs *ChainStore) GetRegistrant(name string) ([]byte, error) {
	return cs.States.getRegistrant(name)
}

func (sdb *StateDB) FinalizeNames(commit bool) {
	for registrantId, name := range sdb.names {
		nameId := getNameId(name)
		registrant := sdb.nameRegistrants[nameId]
		if name == "" {
			sdb.deleteName(registrantId, nameId)
		} else {
			sdb.updateName(registrant, name)
		}
		if commit {
			delete(sdb.names, registrantId)
			delete(sdb.nameRegistrants, nameId)
		}
	}
}
