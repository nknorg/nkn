package db

import (
	"bytes"
	"fmt"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type account struct {
	nonce   uint64
	balance common.Fixed64
}

func NewAccount(n uint64, b common.Fixed64) *account {
	return &account{nonce: n, balance: b}
}

func (acc *account) Serialize(w io.Writer) error {
	if err := serialization.WriteVarUint(w, acc.nonce); err != nil {
		return fmt.Errorf("account nonce Serialize error: %v", err)
	}

	if err := acc.balance.Serialize(w); err != nil {
		return fmt.Errorf("account balance Serialize error: %v", err)
	}

	return nil
}

func (acc *account) Deserialize(r io.Reader) error {
	if acc == nil {
		acc = new(account)
	}

	nonce, err := serialization.ReadVarUint(r, 0)
	if err != nil {
		return fmt.Errorf("Deserialize error:%v", err)
	}

	acc.nonce = nonce
	return acc.balance.Deserialize(r)
}

func (acc *account) GetNonce() uint64 {
	return acc.nonce
}

func (acc *account) GetBalance() common.Fixed64 {
	return acc.balance
}

func (acc *account) SetNonce(nonce uint64) {
	acc.nonce = nonce
}

func (acc *account) SetBalance(balance common.Fixed64) {
	acc.balance = balance
}

func (acc *account) Empty() bool {
	return acc.nonce == 0 && acc.balance == 0
}

type StateDB struct {
	db   *cachingDB
	trie ITrie

	accounts map[common.Uint160]*account
}

func NewStateDB(root common.Uint256, db *cachingDB) (*StateDB, error) {
	trie, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	return &StateDB{
		db:       db,
		trie:     trie,
		accounts: make(map[common.Uint160]*account),
	}, nil
}

func (sdb *StateDB) getAccount(addr common.Uint160) (*account, error) {
	if obj := sdb.accounts[addr]; obj != nil {
		return obj, nil
	}

	enc, err := sdb.trie.TryGet(addr[:])
	if err != nil || len(enc) == 0 {
		return nil, fmt.Errorf("[getAccount]can not get account from trie: %v", err)
	}

	buff := bytes.NewBuffer(enc)
	data := new(account)
	if err := data.Deserialize(buff); err != nil {
		return nil, fmt.Errorf("[getAccount]Failed to decode state object", "addr", addr, "err", err)
	}

	sdb.setAccount(addr, data)

	return data, nil
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

func (sdb *StateDB) setAccount(addr common.Uint160, acc *account) {
	sdb.accounts[addr] = acc
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
	new = NewAccount(0, 0)

	sdb.setAccount(addr, new)
	return new, old
}

func (sdb *StateDB) updateAccount(addr common.Uint160, acc *account) error {
	buff := bytes.NewBuffer(nil)
	err := acc.Serialize(buff)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}

	return sdb.trie.TryUpdate(addr[:], buff.Bytes())
}

func (sdb *StateDB) deleteAccount(addr common.Uint160, acc *account) error {
	err := sdb.trie.TryDelete(addr[:])
	if err != nil {
		return err
	}

	delete(sdb.accounts, addr)
	return nil
}

func (sdb *StateDB) Finalise(deleteEmptyObjects bool) {
	for addr, acc := range sdb.accounts {
		if deleteEmptyObjects && acc.Empty() {
			sdb.deleteAccount(addr, acc)
		} else {
			sdb.updateAccount(addr, acc)
		}
	}
}

func (sdb *StateDB) IntermediateRoot(deleteEmptyObjects bool) common.Uint256 {
	sdb.Finalise(deleteEmptyObjects)
	return sdb.trie.Hash()
}

func (sdb *StateDB) CommitTo(deleteEmptyObjects bool) (root common.Uint256, err error) {
	for addr, acc := range sdb.accounts {
		if deleteEmptyObjects && acc.Empty() {
			sdb.deleteAccount(addr, acc)
		} else {
			sdb.updateAccount(addr, acc)
		}
		delete(sdb.accounts, addr)
	}

	root, err = sdb.trie.CommitTo()

	return root, err
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
