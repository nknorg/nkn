package db

import (
	"sync"

	"github.com/nknorg/nkn/common"
)

var (
	AccountPrefix        = []byte{0x00}
	NanoPayPrefix        = []byte{0x01}
	NanoPayCleanupPrefix = []byte{0x02}
	NamePrefix           = []byte{0x03}
	NameRegistrantPrefix = []byte{0x04}
	PubSubPrefix         = []byte{0x05}
	PubSubCleanupPrefix  = []byte{0x06}
	IssueAssetPrefix     = []byte{0x07}
)

type StateDB struct {
	db              *cachingDB
	trie            ITrie
	accounts        sync.Map
	nanoPay         sync.Map
	nanoPayCleanup  sync.Map
	names           sync.Map
	nameRegistrants sync.Map
	pubSub          sync.Map
	pubSubCleanup   sync.Map
	assets          sync.Map
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

func (sdb *StateDB) Finalize(commit bool) (root common.Uint256, err error) {
	sdb.FinalizeAccounts(commit)

	sdb.FinalizeNanoPay(commit)

	sdb.FinalizeNames(commit)

	sdb.FinalizePubSub(commit)

	sdb.FinalizeIssueAsset(commit)

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
