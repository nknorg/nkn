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

	sdb.names.Store(registrantId, name)
	sdb.nameRegistrants.Store(nameId, registrant)
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

	sdb.names.Delete(registrantId)
	sdb.nameRegistrants.Delete(nameId)

	return nil
}

func (sdb *StateDB) deleteNameForRegistrant(registrant []byte, name string) {
	registrantId := getRegistrantId(registrant)
	nameId := getNameId(name)

	sdb.names.Store(registrantId, "")
	sdb.nameRegistrants.Store(nameId, nil)
}

func (sdb *StateDB) getName(registrant []byte) (string, error) {
	registrantId := getRegistrantId(registrant)

	if v, ok := sdb.names.Load(registrantId); ok {
		if name, ok := v.(string); ok {
			return name, nil
		}
	}

	var name string
	enc, err := sdb.trie.TryGet(append(NameRegistrantPrefix, registrantId...))
	if err != nil {
		return "", err
	}

	if len(enc) > 0 {
		name = string(enc)
		nameId := getNameId(name)
		sdb.names.Store(registrantId, name)
		sdb.nameRegistrants.Store(nameId, registrant)
	}

	return name, nil
}

func (sdb *StateDB) getRegistrant(name string) ([]byte, error) {
	nameId := getNameId(name)

	if v, ok := sdb.nameRegistrants.Load(nameId); ok {
		if registrant, ok := v.([]byte); ok {
			return registrant, nil
		}
	}

	var registrant []byte
	enc, err := sdb.trie.TryGet(append(NamePrefix, nameId...))
	if err != nil {
		return nil, err
	}

	if len(enc) > 0 {
		registrant = enc
		registrantId := getRegistrantId(registrant)
		sdb.names.Store(registrantId, name)
		sdb.nameRegistrants.Store(nameId, registrant)
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
	sdb.names.Range(func(key, value interface{}) bool {
		if registrantId, ok := key.(string); ok {
			if name, ok := value.(string); ok && len(name) > 0 {
				nameId := getNameId(name)
				if v, ok := sdb.nameRegistrants.Load(nameId); ok {
					if registrant, ok := v.([]byte); ok {
						sdb.updateName(registrant, name)
						if commit {
							sdb.nameRegistrants.Delete(nameId)
						}
					}
				}
			} else {
				sdb.deleteName(registrantId)
			}
			if commit {
				sdb.names.Delete(registrantId)
			}
		}
		return true
	})
}
