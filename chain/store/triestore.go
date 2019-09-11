package store

import (
	"github.com/nknorg/nkn/chain/db"
	"github.com/nknorg/nkn/chain/trie"
	. "github.com/nknorg/nkn/common"
)

const (
	maxPastTries = 12
)

type ITrie interface {
	TryGet(key []byte) ([]byte, error)
	TryUpdate(key, value []byte) error
	TryDelete(key []byte) error
	TryTraverse() error
	Hash() Uint256
	CommitTo() (Uint256, error)
	NodeIterator(start []byte) trie.NodeIterator
	NewRefCounts(refCountTargetHeight, pruningTargetHeight uint32) *trie.RefCounts
}

type cachingDB struct {
	db        db.IStore
	pastTries []*trie.Trie
}

func NewTrieStore(db db.IStore) *cachingDB {
	return &cachingDB{db: db}
}

func (db *cachingDB) OpenTrie(root Uint256) (ITrie, error) {
	for i := len(db.pastTries) - 1; i >= 0; i-- {
		h := db.pastTries[i].Hash()
		if h.CompareTo(root) == 0 {
			return cachedTrie{db.pastTries[i].Copy(), db}, nil
		}
	}
	tr, err := trie.New(root, db.db)
	if err != nil {
		return nil, err
	}
	return cachedTrie{tr, db}, nil
}

func (db *cachingDB) pushTrie(t *trie.Trie) {
	if len(db.pastTries) > maxPastTries {
		copy(db.pastTries, db.pastTries[1:])
		db.pastTries[len(db.pastTries)-1] = t
	} else {
		db.pastTries = append(db.pastTries, t)
	}
}

type cachedTrie struct {
	*trie.Trie
	*cachingDB
}

func (c cachedTrie) CommitTo() (Uint256, error) {
	root, err := c.Trie.CommitTo(c.cachingDB.db)
	if err != nil {
		return Uint256{}, err
	}
	c.cachingDB.pushTrie(c.Trie)
	return root, nil
}
