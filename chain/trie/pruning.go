package trie

import (
	"encoding/binary"
	"fmt"
	"reflect"

	"github.com/nknorg/nkn/v2/chain/db"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/util/log"
)

const (
	maxRefCountKeysInMemory = 32768
)

type RefCounts struct {
	trie   *Trie
	counts map[common.Uint256]uint32

	targetRefCountHeight uint32
	targetPruningHeight  uint32
}

func NewRefCounts(t *Trie, refCountTargetHeight, pruningTargetHeight uint32) (*RefCounts, error) {
	ref := &RefCounts{
		counts:               make(map[common.Uint256]uint32, 0),
		trie:                 t,
		targetRefCountHeight: refCountTargetHeight,
		targetPruningHeight:  pruningTargetHeight,
	}

	return ref, nil
}

func (ref *RefCounts) NewBatch() error {
	return ref.trie.db.NewBatch()
}

func (ref *RefCounts) Commit() error {
	return ref.trie.db.BatchCommit()
}

func (ref *RefCounts) Compact() error {
	return ref.trie.db.Compact()
}

func (ref *RefCounts) RebuildRefCount() error {
	iter := ref.trie.db.NewIterator([]byte{byte(db.TRIE_RefCount)})
	for iter.Next() {
		hs, _ := common.Uint256ParseFromBytes(iter.Key()[1:])
		height := binary.LittleEndian.Uint32(iter.Value())
		ref.counts[hs] = height
	}
	iter.Release()

	return nil
}

func (ref *RefCounts) RemoveAllRefCount() error {
	err := ref.trie.db.NewBatch()
	if err != nil {
		return err
	}

	iter := ref.trie.db.NewIterator([]byte{byte(db.TRIE_RefCount)})
	deleted := 0
	for iter.Next() {
		ref.trie.db.BatchDelete(iter.Key())
		deleted++
	}
	iter.Release()

	err = ref.trie.db.BatchCommit()
	if err != nil {
		return err
	}

	log.Infof("Delete %d ref count", deleted)

	return nil
}

func (ref *RefCounts) LengthOfCounts() int {
	return len(ref.counts)
}

func (ref *RefCounts) CreateRefCounts(hash common.Uint256, inMemory, persistIntermediate bool) error {
	root, err := ref.trie.resolveHash(hash.ToArray(), true)
	if err != nil {
		return err
	}

	ref.trie.root = root
	err = ref.createRefCounts(ref.trie.root, inMemory, persistIntermediate)
	if err != nil {
		return err
	}

	return nil
}

func (ref *RefCounts) createRefCounts(n node, inMemory, persistIntermediate bool) error {
	if persistIntermediate && len(ref.counts) >= maxRefCountKeysInMemory {
		err := ref.PersistRefCounts()
		if err != nil {
			return err
		}

		err = ref.Commit()
		if err != nil {
			return err
		}

		log.Infof("Persist %d ref count", len(ref.counts))

		for k := range ref.counts {
			delete(ref.counts, k)
		}

		err = ref.NewBatch()
		if err != nil {
			return err
		}
	}

	switch n := n.(type) {
	case *shortNode:
		hash, _ := n.cache()
		hs, _ := common.Uint256ParseFromBytes(hash)
		count, ok := ref.counts[hs]
		if !inMemory {
			if !ok {
				v, err := ref.trie.db.Get(db.TrieRefCountKey(hash))
				if err == nil {
					count = binary.LittleEndian.Uint32(v)
				}
			}
			ref.counts[hs] = count
		}
		if count == 0 {
			if err := ref.createRefCounts(n.Val, inMemory, persistIntermediate); err != nil {
				return err
			}
		}

		ref.counts[hs]++
		return nil
	case *fullNode:
		hash, _ := n.cache()
		hs, _ := common.Uint256ParseFromBytes(hash)
		count, ok := ref.counts[hs]
		if !inMemory {
			if !ok {
				v, err := ref.trie.db.Get(db.TrieRefCountKey(hash))
				if err == nil {
					count = binary.LittleEndian.Uint32(v)
				}
			}
			ref.counts[hs] = count
		}
		if count == 0 {
			for i := 0; i < LenOfChildrenNodes; i++ {
				if n.Children[i] != nil {
					err := ref.createRefCounts(n.Children[i], inMemory, persistIntermediate)
					if err != nil {
						return err
					}
				}
			}
		}

		ref.counts[hs]++
		return nil
	case hashNode:
		hs, _ := common.Uint256ParseFromBytes(n)
		count, ok := ref.counts[hs]
		if !inMemory {
			if !ok {
				v, err := ref.trie.db.Get(db.TrieRefCountKey(n))
				if err == nil {
					count = binary.LittleEndian.Uint32(v)
				}
			}
			ref.counts[hs] = count
		}
		if count != 0 {
			ref.counts[hs]++
			return nil
		}
		child, err := ref.trie.resolveHash(n, true)
		if err != nil {
			return err
		}
		return ref.createRefCounts(child, inMemory, persistIntermediate)
	case nil:
		return nil
	case valueNode:
		return nil
	default:
		log.Fatalf("Invalid node type : %v, %v", reflect.TypeOf(n), n)
	}
	return nil
}

func (ref *RefCounts) Prune(hash common.Uint256, inMemory bool) error {
	root, err := ref.trie.resolveHash(hash.ToArray(), true)
	if err != nil {
		return err
	}
	ref.trie.root = root

	err = ref.prune(ref.trie.root, inMemory)
	if err != nil {
		return err
	}

	return nil
}

func (ref *RefCounts) prune(n node, inMemory bool) error {
	switch n := n.(type) {
	case *shortNode:
		hash, _ := n.cache()
		hs, _ := common.Uint256ParseFromBytes(hash)
		count, ok := ref.counts[hs]
		if !inMemory {
			if !ok {
				v, err := ref.trie.db.Get(db.TrieRefCountKey(hash))
				if err == nil {
					count = binary.LittleEndian.Uint32(v)
					ref.counts[hs] = count
				} else {
					log.Fatal("Trie get error: %v", err)
				}
			}
		}
		if count == 0 {
			log.Fatalf("RefCount cannot be zero, %v", hs.ToHexString())
		}

		if count > 1 {
			ref.counts[hs] = count - 1
			return nil
		}

		delete(ref.counts, hs)
		ref.trie.db.BatchDelete(db.TrieNodeKey(hash))
		if !inMemory {
			ref.trie.db.BatchDelete(db.TrieRefCountKey(hash))
		}
		return ref.prune(n.Val, inMemory)

	case *fullNode:
		hash, _ := n.cache()
		hs, _ := common.Uint256ParseFromBytes(hash)
		count, ok := ref.counts[hs]
		if !inMemory {
			if !ok {
				v, err := ref.trie.db.Get(db.TrieRefCountKey(hash))
				if err == nil {
					count = binary.LittleEndian.Uint32(v)
					ref.counts[hs] = count
				} else {
					log.Fatal("Trie get error: %v", err)
				}
			}
		}
		if count == 0 {
			log.Fatalf("RefCount cannot be zero, %v", hs.ToHexString())
		}

		if count > 1 {
			ref.counts[hs] = count - 1
			return nil
		}

		delete(ref.counts, hs)
		ref.trie.db.BatchDelete(db.TrieNodeKey(hash))
		if !inMemory {
			ref.trie.db.BatchDelete(db.TrieRefCountKey(hash))
		}
		for i := 0; i < LenOfChildrenNodes; i++ {
			if n.Children[i] != nil {
				if err := ref.prune(n.Children[i], inMemory); err != nil {
					return err
				}
			}
		}

		return nil
	case hashNode:
		hs, _ := common.Uint256ParseFromBytes([]byte(n))
		count, ok := ref.counts[hs]
		if !inMemory {
			if !ok {
				v, err := ref.trie.db.Get(db.TrieRefCountKey([]byte(n)))
				if err == nil {
					count = binary.LittleEndian.Uint32(v)
					ref.counts[hs] = count
				} else {
					log.Fatal("Trie get error: %v", err)
				}
			}
		}
		if count == 0 {
			log.Fatalf("RefCount cannot be zero, %v", hs.ToHexString())
		}

		if count > 1 {
			ref.counts[hs] = count - 1
			return nil
		}

		child, err := ref.trie.resolveHash(n, true)
		if err != nil {
			return err
		}
		return ref.prune(child, inMemory)
	case nil:
		return nil
	case valueNode:
		return nil
	default:
		log.Fatalf("Invalid node type : %v, %v", reflect.TypeOf(n), n)
	}
	return nil
}

func (ref *RefCounts) SequentialPrune() error {
	iter := ref.trie.db.NewIterator([]byte{byte(db.TRIE_Node)})

	for iter.Next() {
		hs, _ := common.Uint256ParseFromBytes(iter.Key()[1:])
		if count := ref.counts[hs]; count == 0 {
			err := ref.trie.db.BatchDelete(iter.Key())
			if err != nil {
				iter.Release()
				return err
			}
		}
	}

	iter.Release()

	return nil
}

func (ref *RefCounts) PersistRefCounts() error {
	for k, v := range ref.counts {
		count := make([]byte, 4)
		binary.LittleEndian.PutUint32(count[:], v)
		err := ref.trie.db.BatchPut(db.TrieRefCountKey(k[:]), count)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ref *RefCounts) PersistRefCountHeights() error {
	heightBuffer := make([]byte, 4)
	binary.LittleEndian.PutUint32(heightBuffer[:], ref.targetRefCountHeight)
	return ref.trie.db.BatchPut(db.TrieRefCountHeightKey(), heightBuffer)
}

func (ref *RefCounts) PersistPrunedHeights() error {
	heightBuffer := make([]byte, 4)
	binary.LittleEndian.PutUint32(heightBuffer[:], ref.targetPruningHeight)
	return ref.trie.db.BatchPut(db.TriePrunedHeightKey(), heightBuffer)
}

func (ref *RefCounts) PersistNeedReset() error {
	return ref.trie.db.BatchPut(db.TrieRefCountNeedResetKey(), []byte{1})
}

func (ref *RefCounts) ClearNeedReset() error {
	return ref.trie.db.BatchDelete(db.TrieRefCountNeedResetKey())
}

func (ref *RefCounts) NeedReset() (bool, error) {
	return ref.trie.db.Has(db.TrieRefCountNeedResetKey())
}

func (ref *RefCounts) Verify(hash common.Uint256) error {
	root, err := ref.trie.resolveHash(hash.ToArray(), false)
	if err != nil {
		return err
	}

	ref.trie.root = root

	err = ref.trie.traverse(ref.trie.root, false)
	if err != nil {
		log.Fatal("Trie traverse error: %v", err)
	}

	hs := ref.trie.Hash()
	if hash.CompareTo(hs) != 0 {
		return fmt.Errorf("state root not equal: %v, %v", hash.ToHexString(), hs.ToHexString())
	}

	return nil
}
