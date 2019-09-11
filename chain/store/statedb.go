package store

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

func (sdb *StateDB) PruneStates(refStateRoots, pruningStateRoots []common.Uint256, refCountTargetHeight, pruningTargetHeight uint32) error {
	refCounts := sdb.trie.NewRefCounts(refCountTargetHeight, pruningTargetHeight)
	refCounts.RebuildRefCount()

	for idx, hash := range refStateRoots {
		err := refCounts.CreateRefCounts(hash)
		if err != nil {
			return err
		}
		refCounts.DumpInfo(uint32(idx), false)
	}

	err := sdb.db.db.NewBatch()
	if err != nil {
		return err
	}

	for idx, hash := range pruningStateRoots {
		err := refCounts.Prune(hash)
		if err != nil {
			return nil
		}
		refCounts.DumpInfo(uint32(idx), true)
	}

	err = refCounts.PersistRefCounts()
	if err != nil {
		return err
	}

	err = refCounts.PersistPruningHeights()
	if err != nil {
		return err
	}

	err = sdb.db.db.BatchCommit()
	if err != nil {
		return err
	}

	err = sdb.db.db.Compact()
	if err != nil {
		return err
	}

	latestStateRoot := refStateRoots[len(refStateRoots)-1]
	err = refCounts.Verify(latestStateRoot)
	if err != nil {
		return err
	}

	return nil
}

func (sdb *StateDB) TrieTraverse() error {
	return sdb.trie.TryTraverse()
}

func (sdb *StateDB) SequentialPrune(refStateRoots []common.Uint256, refCountTargetHeight, pruningTargetHeight uint32) error {
	refCounts := sdb.trie.NewRefCounts(refCountTargetHeight, pruningTargetHeight)

	for idx, hash := range refStateRoots {
		err := refCounts.CreateRefCounts(hash)
		if err != nil {
			return err
		}

		refCounts.DumpInfo(uint32(idx), false)
	}

	err := sdb.db.db.NewBatch()
	if err != nil {
		return err
	}

	err = refCounts.SequentialPrune()
	if err != nil {
		return err
	}

	refCounts.DumpInfo(uint32(0), true)

	err = refCounts.PersistRefCounts()
	if err != nil {
		return err
	}

	err = refCounts.PersistPruningHeights()
	if err != nil {
		return err
	}

	err = sdb.db.db.BatchCommit()
	if err != nil {
		return err
	}

	err = sdb.db.db.Compact()
	if err != nil {
		return err
	}

	latestStateRoot := refStateRoots[len(refStateRoots)-1]
	err = refCounts.Verify(latestStateRoot)
	if err != nil {
		return err
	}

	return nil
}
