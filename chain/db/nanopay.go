package db

import (
	"bytes"
	"fmt"
	"io"
	"sort"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

func getNanoPayId(sender, recipient common.Uint160, nonce uint64) string {
	buf := new(bytes.Buffer)
	_, _ = sender.Serialize(buf)
	_, _ = recipient.Serialize(buf)
	_ = serialization.WriteUint64(buf, nonce)
	return string(buf.Bytes())
}

func getNanoPayCleanupId(height uint32) string {
	buf := new(bytes.Buffer)
	_ = serialization.WriteUint32(buf, height)
	return string(buf.Bytes())
}

type nanoPay struct {
	balance   common.Fixed64
	expiresAt uint32
}

func (np *nanoPay) Serialize(w io.Writer) error {
	err := np.balance.Serialize(w)
	if err != nil {
		return fmt.Errorf("nanoPay Serialize error: %v", err)
	}

	err = serialization.WriteUint32(w, np.expiresAt)
	if err != nil {
		return fmt.Errorf("nanoPay Serialize error: %v", err)
	}

	return nil
}

func (np *nanoPay) Deserialize(r io.Reader) error {
	err := np.balance.Deserialize(r)
	if err != nil {
		return fmt.Errorf("Deserialize nanoPay error: %v", err)
	}

	np.expiresAt, err = serialization.ReadUint32(r)
	if err != nil {
		return fmt.Errorf("Deserialize nanoPay error: %v", err)
	}

	return nil
}

func (np *nanoPay) Empty() bool {
	return np.balance == 0 && np.expiresAt == 0
}

func (sdb *StateDB) getNanoPay(id string) (*nanoPay, error) {
	var np *nanoPay
	var ok bool
	if np, ok = sdb.nanoPay[id]; !ok {
		enc, err := sdb.trie.TryGet(append(NanoPayPrefix, id...))
		if err != nil {
			return nil, err
		}

		np = &nanoPay{}

		if len(enc) > 0 {
			buff := bytes.NewBuffer(enc)
			if err := np.Deserialize(buff); err != nil {
				return nil, fmt.Errorf("[getNanoPay]Failed to decode state object for nano pay: %v", err)
			}
		}

		sdb.nanoPay[id] = np
	}

	return np, nil
}

func (sdb *StateDB) getNanoPayCleanup(height uint32) (map[string]struct{}, error) {
	var npc map[string]struct{}
	var ok bool
	if npc, ok = sdb.nanoPayCleanup[height]; !ok {
		enc, err := sdb.trie.TryGet(append(NanoPayCleanupPrefix, getNanoPayCleanupId(height)...))
		if err != nil || len(enc) == 0 {
			return nil, fmt.Errorf("[getNanoPayCleanup]can not get nano pay cleanup from trie: %v", err)
		}

		buff := bytes.NewBuffer(enc)
		npc = make(map[string]struct{}, 0)
		npcLength, err := serialization.ReadVarUint(buff, 0)
		if err != nil {
			return nil, fmt.Errorf("[getNanoPayCleanup]Failed to decode state object for nano pay cleanup: %v", err)
		}
		for i := uint64(0); i < npcLength; i++ {
			id, err := serialization.ReadVarString(buff)
			if err != nil {
				return nil, fmt.Errorf("[getNanoPayCleanup]Failed to decode state object for nano pay cleanup: %v", err)
			}
			npc[id] = struct{}{}
		}

		sdb.nanoPayCleanup[height] = npc
	}

	return npc, nil
}

func (sdb *StateDB) GetNanoPay(sender, recipient common.Uint160, nonce uint64) (common.Fixed64, uint32, error) {
	id := getNanoPayId(sender, recipient, nonce)

	np, err := sdb.getNanoPay(id)
	if err != nil {
		return 0, 0, err
	}

	return np.balance, np.expiresAt, nil
}

func (sdb *StateDB) updateNanoPay(id string, nanoPay *nanoPay) error {
	buff := bytes.NewBuffer(nil)
	err := nanoPay.Serialize(buff)
	if err != nil {
		panic(fmt.Errorf("can't encode nano pay %v: %v", nanoPay, err))
	}

	return sdb.trie.TryUpdate(append(NanoPayPrefix, id...), buff.Bytes())
}

func (sdb *StateDB) deleteNanoPay(id string) error {
	err := sdb.trie.TryDelete(append(NanoPayPrefix, id...))
	if err != nil {
		return err
	}

	delete(sdb.nanoPay, id)
	return nil
}

func (sdb *StateDB) updateNanoPayCleanup(height uint32, nanoPayCleanup map[string]struct{}) error {
	buff := bytes.NewBuffer(nil)

	if err := serialization.WriteVarUint(buff, uint64(len(nanoPayCleanup))); err != nil {
		panic(fmt.Errorf("can't encode nano pay cleanup %v: %v", nanoPayCleanup, err))
	}
	npcs := make([]string, len(nanoPayCleanup))
	for id := range nanoPayCleanup {
		npcs = append(npcs, id)
	}
	sort.Strings(npcs)
	for _, id := range npcs {
		if err := serialization.WriteVarString(buff, id); err != nil {
			panic(fmt.Errorf("can't encode nano pay cleanup %v: %v", nanoPayCleanup, err))
		}
	}

	return sdb.trie.TryUpdate(append(NanoPayCleanupPrefix, getNanoPayCleanupId(height)...), buff.Bytes())
}

func (sdb *StateDB) deleteNanoPayCleanup(height uint32) error {
	err := sdb.trie.TryDelete(append(NanoPayCleanupPrefix, getNanoPayCleanupId(height)...))
	if err != nil {
		return err
	}

	delete(sdb.nanoPayCleanup, height)
	return nil
}

func (sdb *StateDB) cleanupNanoPayAtHeight(height uint32, id string) error {
	ids, err := sdb.getNanoPayCleanup(height)
	if err != nil {
		return err
	}
	ids[id] = struct{}{}
	return nil
}

func (sdb *StateDB) cancelNanoPayCleanupAtHeight(height uint32, id string) error {
	ids, err := sdb.getNanoPayCleanup(height)
	if err != nil {
		return err
	}
	if _, ok := ids[id]; ok {
		delete(ids, id)
	}
	return nil
}

func (sdb *StateDB) SetNanoPay(sender, recipient common.Uint160, nonce uint64, balance common.Fixed64, expiresAt uint32) error {
	id := getNanoPayId(sender, recipient, nonce)
	var np *nanoPay
	var ok bool
	if np, ok = sdb.nanoPay[id]; !ok {
		np = &nanoPay{balance, expiresAt}
		sdb.nanoPay[id] = np
		if err := sdb.cleanupNanoPayAtHeight(expiresAt, id); err != nil {
			return err
		}
	} else {
		if err := sdb.cancelNanoPayCleanupAtHeight(np.expiresAt, id); err != nil {
			return err
		}
		if err := sdb.cleanupNanoPayAtHeight(expiresAt, id); err != nil {
			return err
		}
		np.balance = balance
		np.expiresAt = expiresAt
	}
	return nil
}

func (sdb *StateDB) CleanupNanoPay(height uint32) error {
	ids, err := sdb.getNanoPayCleanup(height)
	if err != nil {
		return err
	}
	for id := range ids {
		sdb.nanoPay[id] = nil
	}
	sdb.nanoPayCleanup[height] = nil

	return nil
}

func (sdb *StateDB) FinalizeNanoPay(commit bool) {
	for id, nanoPay := range sdb.nanoPay {
		if nanoPay == nil || nanoPay.Empty() {
			sdb.deleteNanoPay(id)
		} else {
			sdb.updateNanoPay(id, nanoPay)
		}
		if commit {
			delete(sdb.nanoPay, id)
		}
	}

	for height, nanoPayCleanup := range sdb.nanoPayCleanup {
		if nanoPayCleanup == nil || len(nanoPayCleanup) == 0 {
			sdb.deleteNanoPayCleanup(height)
		} else {
			sdb.updateNanoPayCleanup(height, nanoPayCleanup)
		}
		if commit {
			delete(sdb.nanoPayCleanup, height)
		}
	}
}
