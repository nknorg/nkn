package store

import (
	"github.com/nknorg/nkn/v2/chain/db"
	"github.com/nknorg/nkn/v2/chain/trie"
	"github.com/nknorg/nkn/v2/common"
)

const (
	maxPastTries = 12
)

type ITrie interface {
	TryGet(key []byte) ([]byte, error)
	TryUpdate(key, value []byte) error
	TryDelete(key []byte) error
	TryTraverse(needPrint bool) error
	Hash() common.Uint256
	CommitTo() (common.Uint256, error)
	NodeIterator(start []byte) trie.NodeIterator
	NewRefCounts(refCountTargetHeight, pruningTargetHeight uint32) (*trie.RefCounts, error)
}

type cachingDB struct {
	db        db.IStore
	pastTries []*trie.Trie
}

func NewTrieStore(db db.IStore) *cachingDB {
	return &cachingDB{db: db}
}

func (db *cachingDB) OpenTrie(root common.Uint256) (ITrie, error) {
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

func (c cachedTrie) CommitTo() (common.Uint256, error) {
	root, err := c.Trie.CommitTo(c.cachingDB.db)
	if err != nil {
		return common.Uint256{}, err
	}
	c.cachingDB.pushTrie(c.Trie)
	return root, nil
}

func (c *cachedTrie) Get(key []byte) ([]byte, error) {
	return c.cachingDB.db.Get(key)
}
