package db

import (
	"strings"
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

	err := sdb.trie.TryUpdate(append(NamePrefix, nameId...), registrant)
	if err != nil {
		return err
	}

	return sdb.trie.TryUpdate(append(NameRegistrantPrefix, registrantId...), []byte(name))
}

func (sdb *StateDB) setName(registrant []byte, name string) {
	registrantId := getRegistrantId(registrant)
	nameId := getNameId(name)

	sdb.names[registrantId] = name
	sdb.nameRegistrants[nameId] = registrant
}

func (sdb *StateDB) deleteName(registrantId string) error {
	name, err := sdb.trie.TryGet(append(NameRegistrantPrefix, registrantId...))
	if err != nil {
		return err
	}

	nameId := getNameId(string(name))
	err = sdb.trie.TryDelete(append(NameRegistrantPrefix, registrantId...))
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

func (sdb *StateDB) deleteNameForRegistrant(registrant []byte, name string) {
	registrantId := getRegistrantId(registrant)
	nameId := getNameId(name)

	sdb.names[registrantId] = ""
	sdb.nameRegistrants[nameId] = nil
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
			name = string(enc)
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
			registrant = enc

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
		if name == "" {
			sdb.deleteName(registrantId)
		} else {
			nameId := getNameId(name)
			registrant := sdb.nameRegistrants[nameId]
			sdb.updateName(registrant, name)
			if commit {
				delete(sdb.nameRegistrants, nameId)
			}
		}
		if commit {
			delete(sdb.names, registrantId)
		}
	}
}
