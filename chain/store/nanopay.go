package store

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

type nanoPayCleanup map[string]struct{}

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
	if v, ok := sdb.nanoPay.Load(id); ok {
		if np, ok := v.(*nanoPay); ok {
			return np, nil
		}
	}

	enc, err := sdb.trie.TryGet(append(NanoPayPrefix, id...))
	if err != nil {
		return nil, err
	}

	np := &nanoPay{}

	if len(enc) > 0 {
		buff := bytes.NewBuffer(enc)
		if err := np.Deserialize(buff); err != nil {
			return nil, fmt.Errorf("[getNanoPay]Failed to decode state object for nano pay: %v", err)
		}
	}

	sdb.nanoPay.Store(id, np)

	return np, nil
}

func (sdb *StateDB) getNanoPayCleanup(height uint32) (nanoPayCleanup, error) {
	if v, ok := sdb.nanoPayCleanup.Load(height); ok {
		if npc, ok := v.(nanoPayCleanup); ok {
			return npc, nil
		}
	}

	enc, err := sdb.trie.TryGet(append(NanoPayCleanupPrefix, getNanoPayCleanupId(height)...))
	if err != nil {
		return nil, fmt.Errorf("[getNanoPayCleanup]can not get nano pay cleanup from trie: %v", err)
	}

	npc := make(nanoPayCleanup, 0)

	if len(enc) > 0 {
		buff := bytes.NewBuffer(enc)
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
	}

	sdb.nanoPayCleanup.Store(height, npc)

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
		return fmt.Errorf("can't encode nano pay %v: %v", nanoPay, err)
	}

	return sdb.trie.TryUpdate(append(NanoPayPrefix, id...), buff.Bytes())
}

func (sdb *StateDB) deleteNanoPay(id string) error {
	err := sdb.trie.TryDelete(append(NanoPayPrefix, id...))
	if err != nil {
		return err
	}

	sdb.nanoPay.Delete(id)
	return nil
}

func (sdb *StateDB) updateNanoPayCleanup(height uint32, npc nanoPayCleanup) error {
	buff := bytes.NewBuffer(nil)

	if err := serialization.WriteVarUint(buff, uint64(len(npc))); err != nil {
		return fmt.Errorf("can't encode nano pay cleanup %v: %v", npc, err)
	}
	npcs := make([]string, 0)
	for id := range npc {
		npcs = append(npcs, id)
	}
	sort.Strings(npcs)
	for _, id := range npcs {
		if err := serialization.WriteVarString(buff, id); err != nil {
			return fmt.Errorf("can't encode nano pay cleanup %v: %v", npc, err)
		}
	}

	return sdb.trie.TryUpdate(append(NanoPayCleanupPrefix, getNanoPayCleanupId(height)...), buff.Bytes())
}

func (sdb *StateDB) deleteNanoPayCleanup(height uint32) error {
	err := sdb.trie.TryDelete(append(NanoPayCleanupPrefix, getNanoPayCleanupId(height)...))
	if err != nil {
		return err
	}

	sdb.nanoPayCleanup.Delete(height)
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
	if v, ok := sdb.nanoPay.Load(id); ok {
		if np, ok := v.(*nanoPay); ok {
			if err := sdb.cancelNanoPayCleanupAtHeight(np.expiresAt, id); err != nil {
				return err
			}
			if err := sdb.cleanupNanoPayAtHeight(expiresAt, id); err != nil {
				return err
			}
			np.balance = balance
			np.expiresAt = expiresAt
			return nil
		}
	}
	np := &nanoPay{
		balance:   balance,
		expiresAt: expiresAt,
	}
	sdb.nanoPay.Store(id, np)
	if err := sdb.cleanupNanoPayAtHeight(expiresAt, id); err != nil {
		return err
	}
	return nil
}

func (sdb *StateDB) CleanupNanoPay(height uint32) error {
	ids, err := sdb.getNanoPayCleanup(height)
	if err != nil {
		return err
	}
	for id := range ids {
		sdb.nanoPay.Store(id, nil)
	}
	sdb.nanoPayCleanup.Store(height, nil)

	return nil
}

func (sdb *StateDB) FinalizeNanoPay(commit bool) error {
	var err error
	sdb.nanoPay.Range(func(key, value interface{}) bool {
		if id, ok := key.(string); ok {
			if np, ok := value.(*nanoPay); ok && !np.Empty() {
				err = sdb.updateNanoPay(id, np)
			} else {
				err = sdb.deleteNanoPay(id)
			}
			if err != nil {
				return false
			}
			if commit {
				sdb.nanoPay.Delete(id)
			}
		}
		return true
	})
	if err != nil {
		return err
	}

	sdb.nanoPayCleanup.Range(func(key, value interface{}) bool {
		if height, ok := key.(uint32); ok {
			if npc, ok := value.(nanoPayCleanup); ok && len(npc) > 0 {
				err = sdb.updateNanoPayCleanup(height, npc)
			} else {
				err = sdb.deleteNanoPayCleanup(height)
			}
			if err != nil {
				return false
			}
			if commit {
				sdb.nanoPayCleanup.Delete(height)
			}
		}
		return true
	})
	return err
}
