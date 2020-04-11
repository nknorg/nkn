package store

import (
	"fmt"
	"sync"

	"github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

var (
	AccountPrefix        = []byte{0x00}
	NanoPayPrefix        = []byte{0x01}
	NanoPayCleanupPrefix = []byte{0x02}
	NamePrefix_legacy    = []byte{0x03}
	NameRegistrantPrefix = []byte{0x04}
	PubSubPrefix         = []byte{0x05}
	PubSubCleanupPrefix  = []byte{0x06}
	IssueAssetPrefix     = []byte{0x07}
	NamePrefix           = []byte{0x08}
	NameCleanupPrefix    = []byte{0x09}
)

type StateDB struct {
	cs              *ChainStore
	db              *cachingDB
	trie            ITrie
	accounts        sync.Map
	nanoPay         sync.Map
	nanoPayCleanup  sync.Map
	names           sync.Map
	nameRegistrants sync.Map
	namesCleanup    sync.Map
	pubSub          sync.Map
	pubSubCleanup   sync.Map
	assets          sync.Map
}

func NewStateDB(root common.Uint256, cs *ChainStore) (*StateDB, error) {
	db := NewTrieStore(cs.GetDatabase())
	trie, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	return &StateDB{
		cs:   cs,
		db:   db,
		trie: trie,
	}, nil
}

func (sdb *StateDB) Finalize(commit bool) (common.Uint256, error) {
	err := sdb.FinalizeAccounts(commit)
	if err != nil {
		return common.EmptyUint256, err
	}

	err = sdb.FinalizeNanoPay(commit)
	if err != nil {
		return common.EmptyUint256, err
	}

	err = sdb.FinalizeNames(commit)
	if err != nil {
		return common.EmptyUint256, err
	}

	err = sdb.FinalizePubSub(commit)
	if err != nil {
		return common.EmptyUint256, err
	}

	err = sdb.FinalizeIssueAsset(commit)
	if err != nil {
		return common.EmptyUint256, err
	}

	if commit {
		return sdb.trie.CommitTo()
	}

	return common.EmptyUint256, nil
}

func (sdb *StateDB) IntermediateRoot() (common.Uint256, error) {
	_, err := sdb.Finalize(false)
	if err != nil {
		return common.EmptyUint256, err
	}
	return sdb.trie.Hash(), nil
}

func (sdb *StateDB) PruneStates() error {
	refCountStartHeight, pruningStartHeight := sdb.cs.getPruningStartHeight()

	_, refCountTargetHeight, err := sdb.cs.getCurrentBlockHashFromDB()
	if err != nil {
		return err
	}

	if refCountStartHeight < pruningStartHeight || refCountTargetHeight < (refCountStartHeight+config.Parameters.RecentStateCount-1) {
		return fmt.Errorf("not enough height to prune, refCountStartHeight:%v, refCountTargetHeight:%v\n", refCountStartHeight, refCountTargetHeight)
	}

	pruningTargetHeight := refCountTargetHeight - config.Parameters.RecentStateCount

	refStateRoots, err := sdb.cs.GetStateRoots(refCountStartHeight, refCountTargetHeight)
	if err != nil {
		return err
	}

	pruningStateRoots, err := sdb.cs.GetStateRoots(pruningStartHeight, pruningTargetHeight)
	if err != nil {
		return err
	}

	refCounts, err := sdb.trie.NewRefCounts(refCountTargetHeight, pruningTargetHeight)
	if err != nil {
		return err
	}

	err = refCounts.NewBatch()
	if err != nil {
		return err
	}

	refCounts.RebuildRefCount()

	for idx, hash := range refStateRoots {
		err = refCounts.CreateRefCounts(hash, true)
		if err != nil {
			return err
		}
		log.Info("refcount height:", refCountStartHeight+uint32(idx), "length of refCounts:", refCounts.LengthOfCounts())
	}

	for idx, hash := range pruningStateRoots {
		err = refCounts.Prune(hash, true)
		if err != nil {
			return err
		}
		log.Info("pruning height:", pruningStartHeight+uint32(idx), "length of refCounts:", refCounts.LengthOfCounts())
	}

	err = refCounts.PersistRefCounts()
	if err != nil {
		return err
	}

	err = refCounts.PersistRefCountHeights()
	if err != nil {
		return err
	}

	err = refCounts.PersistPrunedHeights()
	if err != nil {
		return err
	}

	err = refCounts.Commit()
	if err != nil {
		return err
	}

	err = refCounts.Compact()
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

func (sdb *StateDB) PruneStatesLowMemory() error {
	log.Infof("Start pruning...")

	refCountStartHeight, pruningStartHeight := sdb.cs.getPruningStartHeight()

	_, currentHeight, err := sdb.cs.getCurrentBlockHashFromDB()
	if err != nil {
		return err
	}

	refCountTargetHeight := currentHeight

	if refCountTargetHeight <= config.Parameters.RecentStateCount {
		return nil
	}

	pruningTargetHeight := refCountTargetHeight - config.Parameters.RecentStateCount

	if pruningTargetHeight < pruningStartHeight+config.Parameters.RecentStateCount {
		log.Infof("No need to prune from height %d to %d", pruningStartHeight, pruningTargetHeight)
		return nil
	}

	log.Infof("Creating refCount from height %d to %d", refCountStartHeight, refCountTargetHeight)

	for i := refCountStartHeight; i <= refCountTargetHeight; i++ {
		refStateRoots, err := sdb.cs.GetStateRoots(i, i)
		if err != nil {
			return err
		}
		refCounts, err := sdb.trie.NewRefCounts(i, 0)
		if err != nil {
			return err
		}

		err = refCounts.NewBatch()
		if err != nil {
			return err
		}

		err = refCounts.CreateRefCounts(refStateRoots[0], false)
		if err != nil {
			return err
		}

		err = refCounts.PersistRefCounts()
		if err != nil {
			return err
		}

		err = refCounts.PersistRefCountHeights()
		if err != nil {
			return err
		}

		err = refCounts.Commit()
		if err != nil {
			return err
		}

		log.Info("refcount height:", uint32(i), "length of refCounts:", refCounts.LengthOfCounts())
	}

	log.Infof("Pruning from height %d to %d", pruningStartHeight, pruningTargetHeight)

	for i := pruningStartHeight; i <= pruningTargetHeight; i++ {
		pruningStateRoots, err := sdb.cs.GetStateRoots(i, i)
		if err != nil {
			return err
		}
		refCounts, err := sdb.trie.NewRefCounts(0, i)
		if err != nil {
			return err
		}

		err = refCounts.NewBatch()
		if err != nil {
			return err
		}

		err = refCounts.Prune(pruningStateRoots[0], false)
		if err != nil {
			return err
		}

		err = refCounts.PersistRefCounts()
		if err != nil {
			return err
		}

		err = refCounts.PersistPrunedHeights()
		if err != nil {
			return err
		}

		err = refCounts.Commit()
		if err != nil {
			return err
		}

		log.Info("pruning height:", uint32(i), "length of refCounts:", refCounts.LengthOfCounts())
	}

	refCounts, err := sdb.trie.NewRefCounts(0, 0)
	if err != nil {
		return err
	}

	log.Info("start compact database...")
	err = refCounts.Compact()
	if err != nil {
		return err
	}

	log.Info("start verify database...")
	latestStateRoots, err := sdb.cs.GetStateRoots(refCountTargetHeight, refCountTargetHeight)
	if err != nil {
		return err
	}
	err = refCounts.Verify(latestStateRoots[0])
	if err != nil {
		return err
	}

	return nil
}

func (sdb *StateDB) SequentialPrune() error {
	refCountStartHeight, pruningStartHeight := sdb.cs.getPruningStartHeight()

	_, refCountTargetHeight, err := sdb.cs.getCurrentBlockHashFromDB()
	if err != nil {
		return err
	}

	if refCountStartHeight < pruningStartHeight || refCountTargetHeight < (pruningStartHeight+config.Parameters.RecentStateCount-1) {
		return fmt.Errorf("not enough height to prune, pruningStartHeight:%v, refCountTargetHeight:%v\n", pruningStartHeight, refCountTargetHeight)
	}

	pruningTargetHeight := refCountTargetHeight - config.Parameters.RecentStateCount

	refStateRoots, err := sdb.cs.GetStateRoots(pruningTargetHeight+1, refCountTargetHeight)
	if err != nil {
		return err
	}

	refCounts, err := sdb.trie.NewRefCounts(refCountTargetHeight, pruningTargetHeight)
	if err != nil {
		return err
	}

	err = refCounts.NewBatch()
	if err != nil {
		return err
	}

	for idx, hash := range refStateRoots {
		err := refCounts.CreateRefCounts(hash, true)
		if err != nil {
			return err
		}

		log.Info("refcount height:", pruningTargetHeight+1+uint32(idx), "length of refCounts:", refCounts.LengthOfCounts())
	}

	err = refCounts.SequentialPrune()
	if err != nil {
		return err
	}

	log.Info("pruning height:", pruningTargetHeight, "length of refCounts:", refCounts.LengthOfCounts())

	err = refCounts.PersistRefCounts()
	if err != nil {
		return err
	}

	err = refCounts.PersistRefCountHeights()
	if err != nil {
		return err
	}
	err = refCounts.PersistPrunedHeights()
	if err != nil {
		return err
	}

	err = refCounts.Commit()
	if err != nil {
		return err
	}

	err = refCounts.Compact()
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

func (sdb *StateDB) RollbackPruning(b *block.Block) error {
	refCountStartHeight, pruningStartHeight := sdb.cs.getPruningStartHeight()
	if refCountStartHeight < pruningStartHeight {
		return fmt.Errorf("pruningStartHeight(%v)is larger than refCountStartHeight(%v)\n", pruningStartHeight, refCountStartHeight)
	}

	if refCountStartHeight == 0 {
		return nil
	}

	if refCountStartHeight == pruningStartHeight {
		return fmt.Errorf("Cannot rollback, no more full state available in db, pruningStartHeight(%v)is equal to refCountStartHeight(%v)\n", pruningStartHeight, refCountStartHeight)
	}

	refCountedHeight := refCountStartHeight - 1
	blockHeight := b.Header.UnsignedHeader.Height

	if refCountedHeight < blockHeight {
		return nil
	}

	if refCountedHeight > blockHeight {
		return fmt.Errorf("refCountedHeight(%v) is larger than blockHeight(%v)\n", refCountedHeight, blockHeight)
	}

	refCounts, err := sdb.trie.NewRefCounts(refCountedHeight-1, 0)
	if err != nil {
		return err
	}
	root, err := common.Uint256ParseFromBytes(b.Header.UnsignedHeader.StateRoot)
	if err != nil {
		return err
	}
	err = refCounts.Prune(root, false)
	if err != nil {
		return err
	}
	err = refCounts.PersistRefCounts()
	if err != nil {
		return err
	}

	err = refCounts.PersistRefCountHeights()
	if err != nil {
		return err
	}

	return nil
}

func (sdb *StateDB) VerifyState() error {
	_, refCountTargetHeight, err := sdb.cs.getCurrentBlockHashFromDB()
	if err != nil {
		return err
	}

	refCounts, err := sdb.trie.NewRefCounts(0, 0)
	if err != nil {
		return err
	}

	latestStateRoots, err := sdb.cs.GetStateRoots(refCountTargetHeight, refCountTargetHeight)
	if err != nil {
		return err
	}
	err = refCounts.Verify(latestStateRoots[0])
	if err != nil {
		return err
	}

	return nil
}
