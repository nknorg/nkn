package store

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/nknorg/nkn/util/config"

	"github.com/nknorg/nkn/common"
)

var sdb, _ = defaultStateDB()

func TestStateDB_registerName(t *testing.T) {
	name := "nknorg"
	expiresAt := uint32(10000)
	pubkey, _ := hex.DecodeString("822dd8c6d980bc11a29161c342200b91199846af86c212232f03b78464ca7697")
	err := sdb.registerName(name, pubkey, expiresAt)
	if err != nil {
		t.Fatalf("register error")
	}
	sdb.FinalizeNames(true)
	registrant, ex, _ := sdb.getRegistrant(name)
	if !bytes.Equal(registrant, pubkey) {
		t.Errorf("register's pubkey incorrect, got %x != %x", pubkey, registrant)
	}
	if ex != expiresAt {
		t.Errorf("name's expiresAt incorrect, got %d != %d", expiresAt, ex)
	}
}

func TestStateDB_transferName(t *testing.T) {
	name := "nknorg"
	expiresAt := uint32(10000)
	pubkey, _ := hex.DecodeString("822dd8c6d980bc11a29161c342200b91199846af86c212232f03b78464ca7697")
	err := sdb.registerName(name, pubkey, expiresAt)
	if err != nil {
		t.Fatalf("transfer error")
	}
	sdb.FinalizeNames(true)

	toPubkey := "a6c50a62142e107b3fbbe6f163522ce30e52bf45bd8a47762660265f141b6510"
	to, _ := hex.DecodeString(toPubkey)
	sdb.transferName(name, to)
	sdb.FinalizeNames(true)
	registrant, ex, _ := sdb.getRegistrant(name)
	if !bytes.Equal(to, registrant) {
		t.Errorf("register's pubkey incorrect, got %x != %x", toPubkey, registrant)
	}
	if ex != expiresAt {
		t.Errorf("name's expiresAt incorrect, got %d != %d", expiresAt, ex)
	}
}

func TestStateDB_extensionName(t *testing.T) {
	name := "nknorg"
	expiresAt := uint32(10000)
	pubkey, _ := hex.DecodeString("822dd8c6d980bc11a29161c342200b91199846af86c212232f03b78464ca7697")
	err := sdb.registerName(name, pubkey, expiresAt)
	if err != nil {
		t.Fatalf("register error")
	}
	sdb.FinalizeNames(true)

	err = sdb.registerName(name, pubkey, expiresAt+100)
	if err != nil {
		t.Fatalf("extension error")
	}
	registrant, ex, _ := sdb.getRegistrant(name)
	if !bytes.Equal(pubkey, registrant) {
		t.Errorf("register's pubkey incorrect, got %x != %x", pubkey, registrant)
	}
	if ex != expiresAt+100 {
		t.Errorf("name's expiresAt incorrect, got %d != %d", expiresAt, ex)
	}
}

func TestStateDB_deleteName(t *testing.T) {
	name := "nknorg"
	expiresAt := uint32(10000)
	pubkey, _ := hex.DecodeString("822dd8c6d980bc11a29161c342200b91199846af86c212232f03b78464ca7697")
	err := sdb.registerName(name, pubkey, expiresAt)
	if err != nil {
		t.Fatalf("register error")
	}
	sdb.FinalizeNames(true)

	err = sdb.deleteName(name)
	if err != nil {
		t.Fatalf("register error")
	}
	registrant, ex, _ := sdb.getRegistrant(name)
	if len(registrant) != 0 {
		t.Errorf("registrant not deleted")
	}
	if ex != 0 {
		t.Errorf("expires not deleted")
	}
}

func defaultStateDB() (*StateDB, error) {
	config.Parameters.ChainDBPath = "TestChainDB"
	cs, err := NewLedgerStore()
	if err != nil {
		return nil, err
	}
	sdb, err := NewStateDB(common.EmptyUint256, cs)
	if err != nil {
		return nil, err
	}
	return sdb, nil
}
