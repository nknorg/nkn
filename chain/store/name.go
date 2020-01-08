package store

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/nknorg/nkn/common/serialization"
)

type nameInfo struct {
	registrant []byte
	expiresAt  uint32
}

type namesCleanup map[string]struct{}

func (ni *nameInfo) Serialize(w io.Writer) error {
	if err := serialization.WriteVarBytes(w, ni.registrant); err != nil {
		return err
	}
	if err := serialization.WriteUint32(w, ni.expiresAt); err != nil {
		return err
	}
	return nil
}

func (ni *nameInfo) Deserialize(r io.Reader) error {
	var err error
	ni.registrant, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	ni.expiresAt, err = serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	return nil
}

func (ni *nameInfo) Bytes() ([]byte, error) {
	buff := bytes.NewBuffer(nil)
	err := ni.Serialize(buff)
	if err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

func (ni *nameInfo) String() string {
	return fmt.Sprintf("%064x %d", ni.registrant, ni.expiresAt)
}

func (ni *nameInfo) Empty() bool {
	return len(ni.registrant) == 0 && ni.expiresAt == 0
}

func getNameId(name string) []byte {
	nameId := sha256.Sum256([]byte(strings.ToLower(name)))
	return nameId[:hashPrefixLength]
}

func (sdb *StateDB) getRegistrant(name string) ([]byte, uint32, error) {
	nameId := getNameId(name)
	ni, err := sdb.getNameInfo(nameId)
	if err != nil {
		return nil, 0, fmt.Errorf("name registration info not found, %v", err)
	}

	return ni.registrant, ni.expiresAt, nil
}

func (cs *ChainStore) GetRegistrant(name string) ([]byte, uint32, error) {
	return cs.States.getRegistrant(name)
}

func (sdb *StateDB) getNameInfo(nameId []byte) (*nameInfo, error) {
	if v, ok := sdb.names.Load(string(nameId)); ok {
		if ni, ok := v.(*nameInfo); ok {
			return ni, nil
		}
	}

	enc, err := sdb.trie.TryGet(append(NamePrefix, nameId...))
	if err != nil {
		return nil, err
	}
	ni := new(nameInfo)
	if len(enc) > 0 {
		buff := bytes.NewBuffer(enc)
		if err := ni.Deserialize(buff); err != nil {
			return nil, err
		}
	}

	sdb.names.Store(string(nameId), ni)

	return ni, nil
}

func (sdb *StateDB) registerName(name string, registrant []byte, expiresAt uint32) error {
	nameId := getNameId(name)

	ni, err := sdb.getNameInfo(nameId)
	if err != nil {
		return err
	}

	if !ni.Empty() {
		if err := sdb.cancelNameCleanupAtHeight(ni.expiresAt, name); err != nil {
			return err
		}
	}
	ni.registrant = registrant
	ni.expiresAt = expiresAt

	return sdb.cleanupNamesAtHeight(expiresAt, nameId)
}

func (sdb *StateDB) updateNameInfo(nameId string, ni *nameInfo) error {
	nibytes, err := ni.Bytes()
	if err != nil {
		return err
	}
	return sdb.trie.TryUpdate(append(NamePrefix, nameId...), nibytes)
}

func (sdb *StateDB) deleteNameInfo(nameId string) error {
	err := sdb.trie.TryDelete(append(NamePrefix, nameId...))
	if err != nil {
		return err
	}

	sdb.names.Delete(nameId)
	return nil
}

func (sdb *StateDB) FinalizeNames(commit bool) {
	sdb.names.Range(func(key, value interface{}) bool {
		if nameId, ok := key.(string); ok {
			if info, ok := value.(*nameInfo); ok && !info.Empty() {
				sdb.updateNameInfo(nameId, info)
			} else {
				sdb.deleteNameInfo(nameId)
			}
			if commit {
				sdb.names.Delete(nameId)
			}
		}
		return true
	})

	sdb.namesCleanup.Range(func(key, value interface{}) bool {
		if height, ok := key.(uint32); ok {
			if nc, ok := value.(namesCleanup); ok && len(nc) > 0 {
				sdb.updateNamesCleanup(height, nc)
			} else {
				sdb.deleteNamesCleanup(height)
			}
			if commit {
				sdb.namesCleanup.Delete(height)
			}
		}
		return true
	})
}

func (sdb *StateDB) getNamesCleanup(height uint32) (namesCleanup, error) {
	if v, ok := sdb.namesCleanup.Load(height); ok {
		if nc, ok := v.(namesCleanup); ok {
			return nc, nil
		}
	}

	enc, err := sdb.trie.TryGet(append(NameCleanupPrefix, getNamesCleanupId(height)...))
	if err != nil {
		return nil, fmt.Errorf("[getNamesCleanup]can not get name cleanup from trie: %v", err)
	}

	nc := make(namesCleanup, 0)

	if len(enc) > 0 {
		buff := bytes.NewBuffer(enc)
		ncLength, err := serialization.ReadVarUint(buff, 0)
		if err != nil {
			return nil, fmt.Errorf("[getNamesCleanup]Failed to decode state object for name cleanup: %v", err)
		}
		for i := uint64(0); i < ncLength; i++ {
			id, err := serialization.ReadVarString(buff)
			if err != nil {
				return nil, fmt.Errorf("[getNamesCleanup]Failed to decode state object for name cleanup: %v", err)
			}
			nc[id] = struct{}{}
		}
	}

	sdb.namesCleanup.Store(height, nc)
	return nc, nil
}

func getNamesCleanupId(height uint32) []byte {
	buff := bytes.NewBuffer(nil)
	serialization.WriteUint32(buff, height)
	return buff.Bytes()
}

func (sdb *StateDB) CleanupNames(height uint32) error {
	ids, err := sdb.getNamesCleanup(height)
	if err != nil {
		return err
	}
	for id := range ids {
		sdb.names.Store(id, nil)
	}
	sdb.namesCleanup.Store(height, nil)

	return nil
}

func (sdb *StateDB) cleanupNamesAtHeight(height uint32, id []byte) error {
	ids, err := sdb.getNamesCleanup(height)
	if err != nil {
		return err
	}
	ids[string(id)] = struct{}{}
	return nil
}

func (sdb *StateDB) deleteNamesCleanup(height uint32) error {
	err := sdb.trie.TryDelete(append(NameCleanupPrefix, getNamesCleanupId(height)...))
	if err != nil {
		return err
	}

	sdb.namesCleanup.Delete(height)
	return nil
}

func (sdb *StateDB) updateNamesCleanup(height uint32, nc namesCleanup) error {
	buff := bytes.NewBuffer(nil)

	if err := serialization.WriteVarUint(buff, uint64(len(nc))); err != nil {
		panic(fmt.Errorf("can't encode names cleanup %v: %v", nc, err))
	}
	ncs := make([]string, 0, len(nc))
	for id := range nc {
		ncs = append(ncs, id)
	}
	sort.Strings(ncs)
	for _, id := range ncs {
		if err := serialization.WriteVarString(buff, id); err != nil {
			panic(fmt.Errorf("can't encode names cleanup %v: %v", nc, err))
		}
	}

	return sdb.trie.TryUpdate(append(NameCleanupPrefix, getNamesCleanupId(height)...), buff.Bytes())
}

func (sdb *StateDB) cancelNameCleanupAtHeight(height uint32, name string) error {
	ids, err := sdb.getPubSubCleanup(height)
	if err != nil {
		return err
	}
	delete(ids, name)
	return nil
}
