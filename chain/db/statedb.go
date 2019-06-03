package db

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type Operation int

const (
	Addition Operation = iota
	Subtraction
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
	db       *cachingDB
	trie     ITrie
	accounts sync.Map
}

func NewStateDB(root common.Uint256, db *cachingDB) (*StateDB, error) {
	trie, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	return &StateDB{
		db:   db,
		trie: trie,
	}, nil
}

func (sdb *StateDB) getAccount(addr common.Uint160) (*account, error) {
	if v, ok := sdb.accounts.Load(addr); ok {
		if acc, ok := v.(*account); ok {
			return acc, nil
		}
	}

	enc, err := sdb.trie.TryGet(addr[:])
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

	return sdb.trie.TryUpdate(addr[:], buff.Bytes())
}

func (sdb *StateDB) deleteAccount(addr common.Uint160, acc *account) error {
	err := sdb.trie.TryDelete(addr[:])
	if err != nil {
		return err
	}

	sdb.accounts.Delete(addr)
	return nil
}

func (sdb *StateDB) Finalise(deleteEmptyObjects bool) {
	sdb.accounts.Range(func(key, v interface{}) bool {
		if addr, ok := key.(common.Uint160); ok {
			if acc, ok := v.(*account); ok {
				if deleteEmptyObjects && acc.Empty() {
					sdb.deleteAccount(addr, acc)
				} else {
					sdb.updateAccount(addr, acc)
				}
			}
		}
		return true
	})
}

func (sdb *StateDB) IntermediateRoot(deleteEmptyObjects bool) common.Uint256 {
	sdb.Finalise(deleteEmptyObjects)
	return sdb.trie.Hash()
}

func (sdb *StateDB) CommitTo(deleteEmptyObjects bool) (root common.Uint256, err error) {
	sdb.accounts.Range(func(key, v interface{}) bool {
		if addr, ok := key.(common.Uint160); ok {
			if acc, ok := v.(*account); ok {
				if deleteEmptyObjects && acc.Empty() {
					sdb.deleteAccount(addr, acc)
				} else {
					sdb.updateAccount(addr, acc)
				}
			}
			sdb.accounts.Delete(addr)
		}
		return true
	})

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

func (sdb *StateDB) SetID(addr common.Uint160, id []byte) error {
	account, err := sdb.getAccount(addr)
	if err != nil {
		return err
	}

	account.SetID(id)
	return nil
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
