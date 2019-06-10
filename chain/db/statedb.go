package db

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type Operation int

const (
	Addition Operation = iota
	Subtraction
)

var (
	AccountPrefix        = []byte{0x00}
	NanoPayPrefix        = []byte{0x01}
	NanoPayCleanupPrefix = []byte{0x02}
)

type account struct {
	nonce   uint64
	balance common.Fixed64
	id      []byte
}

func NewAccount(n uint64, b common.Fixed64, id []byte) *account {
	return &account{nonce: n, balance: b, id: id}
}

func (acc *account) Serialize(w io.Writer) error {
	if err := serialization.WriteVarUint(w, acc.nonce); err != nil {
		return fmt.Errorf("account nonce Serialize error: %v", err)
	}

	if err := acc.balance.Serialize(w); err != nil {
		return fmt.Errorf("account balance Serialize error: %v", err)
	}

	if err := serialization.WriteVarBytes(w, acc.id); err != nil {
		return fmt.Errorf("account id Serialize error: %v", err)
	}

	return nil
}

func (acc *account) Deserialize(r io.Reader) error {
	nonce, err := serialization.ReadVarUint(r, 0)
	if err != nil {
		return fmt.Errorf("Deserialize nonce error:%v", err)
	}
	acc.nonce = nonce

	err = acc.balance.Deserialize(r)
	if err != nil {
		return fmt.Errorf("Deserialize balance error:%v", err)
	}

	id, err := serialization.ReadVarBytes(r)
	if err != nil {
		return fmt.Errorf("Deserialize id error:%v", err)
	}
	acc.id = id

	return nil
}

func (acc *account) GetNonce() uint64 {
	return acc.nonce
}

func (acc *account) GetBalance() common.Fixed64 {
	return acc.balance
}

func (acc *account) GetID() []byte {
	return acc.id
}

func (acc *account) SetNonce(nonce uint64) {
	acc.nonce = nonce
}

func (acc *account) SetBalance(balance common.Fixed64) {
	acc.balance = balance
}

func (acc *account) SetID(id []byte) {
	acc.id = id
}

func (acc *account) Empty() bool {
	return acc.nonce == 0 && acc.balance == 0 && acc.id == nil
}

type StateDB struct {
	db             *cachingDB
	trie           ITrie
	accounts       sync.Map
	nanoPay        map[string]*nanoPay
	nanoPayCleanup map[uint32]map[string]struct{}
}

func NewStateDB(root common.Uint256, db *cachingDB) (*StateDB, error) {
	trie, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	return &StateDB{
		db:             db,
		trie:           trie,
		nanoPay:        make(map[string]*nanoPay, 0),
		nanoPayCleanup: make(map[uint32]map[string]struct{}, 0),
	}, nil
}

func (sdb *StateDB) getAccount(addr common.Uint160) (*account, error) {
	if v, ok := sdb.accounts.Load(addr); ok {
		if acc, ok := v.(*account); ok {
			return acc, nil
		}
	}

	enc, err := sdb.trie.TryGet(append(AccountPrefix, addr[:]...))
	if err != nil || len(enc) == 0 {
		return nil, fmt.Errorf("[getAccount]can not get account from trie: %v", err)
	}

	buff := bytes.NewBuffer(enc)
	data := new(account)
	if err := data.Deserialize(buff); err != nil {
		return nil, fmt.Errorf("[getAccount]Failed to decode state object for addr %v: %v", addr, err)
	}

	sdb.setAccount(addr, data)

	return data, nil
}

func getNanoPayId(sender, recipient common.Uint160, nonce uint64) string {
	buf := new(bytes.Buffer)
	_, _ = sender.Serialize(buf)
	_, _ = recipient.Serialize(buf)
	_ = serialization.WriteUint64(buf, nonce)
	return string(buf.Bytes())
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

func getNanoPayCleanupId(height uint32) string {
	buf := new(bytes.Buffer)
	_ = serialization.WriteUint32(buf, height)
	return string(buf.Bytes())
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

func (sdb *StateDB) GetBalance(addr common.Uint160) common.Fixed64 {
	account, err := sdb.getAccount(addr)
	if err != nil {
		return common.Fixed64(0)
	}
	return account.GetBalance()
}

func (sdb *StateDB) GetNonce(addr common.Uint160) uint64 {
	account, err := sdb.getAccount(addr)
	if err != nil {
		return 0
	}

	return account.GetNonce()
}

func (sdb *StateDB) GetID(addr common.Uint160) []byte {
	account, err := sdb.getAccount(addr)
	if err != nil {
		return nil
	}

	return account.GetID()
}

func (sdb *StateDB) GetNanoPay(sender, recipient common.Uint160, nonce uint64) (common.Fixed64, uint32, error) {
	id := getNanoPayId(sender, recipient, nonce)

	np, err := sdb.getNanoPay(id)
	if err != nil {
		return 0, 0, err
	}

	return np.balance, np.expiresAt, nil
}

func (sdb *StateDB) SetAccount(addr common.Uint160, acc *account) {
	sdb.setAccount(addr, acc)
}
func (sdb *StateDB) setAccount(addr common.Uint160, acc *account) {
	sdb.accounts.Store(addr, acc)
}

func (sdb *StateDB) GetOrNewAccount(addr common.Uint160) *account {
	enc, err := sdb.getAccount(addr)
	if err != nil {
		enc, _ = sdb.createAccount(addr)
	}
	return enc
}

func (sdb *StateDB) createAccount(addr common.Uint160) (new, old *account) {
	old, _ = sdb.getAccount(addr)
	new = NewAccount(0, 0, nil)

	sdb.setAccount(addr, new)
	return new, old
}

func (sdb *StateDB) updateAccount(addr common.Uint160, acc *account) error {
	buff := bytes.NewBuffer(nil)
	err := acc.Serialize(buff)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}

	return sdb.trie.TryUpdate(append(AccountPrefix, addr[:]...), buff.Bytes())
}

func (sdb *StateDB) deleteAccount(addr common.Uint160) error {
	err := sdb.trie.TryDelete(append(AccountPrefix, addr[:]...))
	if err != nil {
		return err
	}

	sdb.accounts.Delete(addr)
	return nil
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

func (sdb *StateDB) Finalize(commit bool) (root common.Uint256, err error) {
	sdb.accounts.Range(func(key, v interface{}) bool {
		if addr, ok := key.(common.Uint160); ok {
			if acc, ok := v.(*account); ok {
				if acc.Empty() {
					sdb.deleteAccount(addr)
				} else {
					sdb.updateAccount(addr, acc)
				}
			}
			if commit {
				sdb.accounts.Delete(addr)
			}
		}
		return true
	})

	for id, nanoPay := range sdb.nanoPay {
		if nanoPay == nil {
			sdb.deleteNanoPay(id)
		} else {
			sdb.updateNanoPay(id, nanoPay)
		}
		if commit {
			delete(sdb.nanoPay, id)
		}
	}

	for height, nanoPayCleanup := range sdb.nanoPayCleanup {
		if nanoPayCleanup == nil {
			sdb.deleteNanoPayCleanup(height)
		} else {
			sdb.updateNanoPayCleanup(height, nanoPayCleanup)
		}
		if commit {
			delete(sdb.nanoPayCleanup, height)
		}
	}

	if commit {
		root, err = sdb.trie.CommitTo()

		return root, err
	}

	return common.EmptyUint256, nil
}

func (sdb *StateDB) IntermediateRoot() common.Uint256 {
	sdb.Finalize(false)
	return sdb.trie.Hash()
}

func (sdb *StateDB) SetBalance(addr common.Uint160, value common.Fixed64) error {
	account, err := sdb.getAccount(addr)
	if err != nil {
		return err
	}

	account.SetBalance(value)
	return nil
}

func (sdb *StateDB) SetNonce(addr common.Uint160, nonce uint64) error {
	account, err := sdb.getAccount(addr)
	if err != nil {
		return err
	}

	account.SetNonce(nonce)
	return nil
}

func (sdb *StateDB) SetID(addr common.Uint160, id []byte) error {
	account, err := sdb.getAccount(addr)
	if err != nil {
		return err
	}

	account.SetID(id)
	return nil
}

func (sdb *StateDB) cleanupNanoPayAtHeight(height uint32, id string) {
	ids, ok := sdb.nanoPayCleanup[height]
	if !ok {
		ids = make(map[string]struct{})
		sdb.nanoPayCleanup[height] = ids
	}
	ids[id] = struct{}{}
}

func (sdb *StateDB) cancelNanoPayCleanupAtHeight(height uint32, id string) {
	if ids, ok := sdb.nanoPayCleanup[height]; ok {
		if _, ok = ids[id]; ok {
			delete(ids, id)
		}
	}
}

func (sdb *StateDB) SetNanoPay(sender, recipient common.Uint160, nonce uint64, balance common.Fixed64, expiresAt uint32) {
	id := getNanoPayId(sender, recipient, nonce)
	var np *nanoPay
	var ok bool
	if np, ok = sdb.nanoPay[id]; !ok {
		np = &nanoPay{balance, expiresAt}
		sdb.nanoPay[id] = np
		sdb.cleanupNanoPayAtHeight(expiresAt, id)
	} else {
		sdb.cancelNanoPayCleanupAtHeight(np.expiresAt, id)
		sdb.cleanupNanoPayAtHeight(expiresAt, id)
		np.balance = balance
		np.expiresAt = expiresAt
	}
}

func (sdb *StateDB) CleanupNanoPay(height uint32) {
	if ids, ok := sdb.nanoPayCleanup[height]; ok {
		for id := range ids {
			sdb.nanoPay[id] = nil
		}
		sdb.nanoPayCleanup[height] = nil
		delete(sdb.nanoPayCleanup, height)
	}
}

func (sdb *StateDB) UpdateBalance(addr common.Uint160, value common.Fixed64, op Operation) error {
	acc := sdb.GetOrNewAccount(addr)
	amount := acc.GetBalance()

	switch op {
	case Addition:
		acc.SetBalance(amount + value)
	case Subtraction:
		if amount < value {
			return errors.New("UpdateBalance: no sufficient funds")
		}
		acc.SetBalance(amount - value)
	default:
		return errors.New("UpdateBalance: invalid operation")
	}

	return nil
}

func (sdb *StateDB) IncrNonce(addr common.Uint160) error {
	acc := sdb.GetOrNewAccount(addr)
	nonce := acc.GetNonce()
	acc.SetNonce(nonce + 1)

	return nil
}

func (sdb *StateDB) UpdateID(addr common.Uint160, id []byte) error {
	acc := sdb.GetOrNewAccount(addr)
	acc.SetID(id)

	return nil
}
