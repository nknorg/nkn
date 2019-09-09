package store

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type Operation int

const (
	Addition Operation = iota
	Subtraction
)

type sortIDs []common.Uint256

func (s sortIDs) Len() int      { return len(s) }
func (s sortIDs) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortIDs) Less(i, j int) bool {
	if s[i].CompareTo(s[j]) == 1 {
		return false
	} else {
		return true
	}
}

type balance struct {
	assetID common.Uint256
	amount  common.Fixed64
}

func (b *balance) Serialize(w io.Writer) error {
	if _, err := b.assetID.Serialize(w); err != nil {
		return fmt.Errorf("balance asset id serialize error: %v", err)
	}

	if err := b.amount.Serialize(w); err != nil {
		return fmt.Errorf("balance amount serialize error: %v", err)
	}

	return nil
}

func (b *balance) Deserialize(r io.Reader) error {
	var err error

	if err := b.assetID.Deserialize(r); err != nil {
		return fmt.Errorf("balance asset id serialize error: %v", err)
	}

	err = b.amount.Deserialize(r)
	if err != nil {
		return fmt.Errorf("balance amount deserialize error:%v", err)
	}

	return nil
}

type account struct {
	nonce    uint64
	balances map[common.Uint256]*balance
	id       []byte
}

func NewAccount(n uint64, b *balance, id []byte) *account {
	acc := &account{
		nonce:    n,
		balances: make(map[common.Uint256]*balance),
		id:       id,
	}

	if b != nil {
		acc.balances[b.assetID] = b
	}

	return acc
}

func (acc *account) Serialize(w io.Writer) error {
	if err := serialization.WriteVarUint(w, acc.nonce); err != nil {
		return fmt.Errorf("account nonce Serialize error: %v", err)
	}

	var ids []common.Uint256
	for id, _ := range acc.balances {
		ids = append(ids, id)
	}
	sort.Sort(sortIDs(ids))

	if err := serialization.WriteVarUint(w, uint64(len(ids))); err != nil {
		return fmt.Errorf("length of acc.balance error: %v", err)
	}
	for _, id := range ids {
		if err := acc.balances[id].Serialize(w); err != nil {
			return fmt.Errorf("account balance Serialize error: %v", err)
		}
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

	if acc.balances == nil {
		acc.balances = make(map[common.Uint256]*balance)
	}

	balanceSize, err := serialization.ReadVarUint(r, 0)
	if err != nil {
		return fmt.Errorf("Deserialize length of balances error:%v", err)
	}

	for i := 0; i < int(balanceSize); i++ {
		var b balance
		err := b.Deserialize(r)
		if err != nil {
			return fmt.Errorf("Deserialize balances error:%v", err)
		}
		acc.balances[b.assetID] = &b
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

func (acc *account) GetBalance(assetID common.Uint256) common.Fixed64 {
	if _, ok := acc.balances[assetID]; !ok {
		return common.Fixed64(0)
	}

	return acc.balances[assetID].amount
}

func (acc *account) GetID() []byte {
	return acc.id
}

func (acc *account) SetNonce(nonce uint64) {
	acc.nonce = nonce
}

func (acc *account) SetBalance(assetID common.Uint256, amount common.Fixed64) {
	if _, ok := acc.balances[assetID]; !ok {
		acc.balances[assetID] = &balance{
			assetID: assetID,
			amount:  amount,
		}
		return
	}

	acc.balances[assetID].amount = amount
}

func (acc *account) SetID(id []byte) {
	acc.id = id
}

func (acc *account) Empty() bool {
	return acc.nonce == 0 && len(acc.balances) == 0 && acc.id == nil
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

func (sdb *StateDB) GetBalance(assetID common.Uint256, addr common.Uint160) common.Fixed64 {
	account, err := sdb.getAccount(addr)
	if err != nil {
		return common.Fixed64(0)
	}

	return account.GetBalance(assetID)
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
	new = NewAccount(0, nil, nil)

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

func (sdb *StateDB) SetBalance(addr common.Uint160, assetID common.Uint256, value common.Fixed64) error {
	account, err := sdb.getAccount(addr)
	if err != nil {
		return err
	}

	account.SetBalance(assetID, value)
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

func (sdb *StateDB) UpdateBalance(addr common.Uint160, assetID common.Uint256, value common.Fixed64, op Operation) error {
	acc := sdb.GetOrNewAccount(addr)
	amount := acc.GetBalance(assetID)

	switch op {
	case Addition:
		acc.SetBalance(assetID, amount+value)
	case Subtraction:
		if amount < value {
			return errors.New("UpdateBalance: no sufficient funds")
		}
		acc.SetBalance(assetID, amount-value)
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

func (sdb *StateDB) FinalizeAccounts(commit bool) {
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
}
