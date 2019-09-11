package trie

import (
	"encoding/binary"
	"fmt"
	"reflect"

	"github.com/nknorg/nkn/chain/db"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/log"
)

type RefCounts struct {
	trie   *Trie
	counts map[common.Uint256]uint32

	startRefCountHeight  uint32
	startPruningHeight   uint32
	targetRefCountHeight uint32
	targetPruningHeight  uint32

	dbCounter   int32
	nodeCounter int32
}

func NewRefCounts(t *Trie, refCountTargetHeight, pruningTargetHeight uint32) *RefCounts {
	ref := &RefCounts{
		counts:               make(map[common.Uint256]uint32, 0),
		trie:                 t,
		targetRefCountHeight: refCountTargetHeight,
		targetPruningHeight:  pruningTargetHeight,
	}

	heightBuffer, err := t.db.Get(db.TriePrunedHeightKey())
	if err != nil {
		log.Info("can not get current pruned height from DB,", err)
		ref.startPruningHeight = 0
	} else {
		ref.startPruningHeight = binary.LittleEndian.Uint32(heightBuffer) + 1
	}

	heightBuffer, err = t.db.Get(db.TrieRefCountHeightKey())
	if err != nil {
		log.Info("can not get current counted height from DB,", err)
		ref.startRefCountHeight = 0
	} else {
		ref.startRefCountHeight = binary.LittleEndian.Uint32(heightBuffer) + 1
	}

	return ref
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

func (ref *RefCounts) DumpInfo(offset uint32, pruning bool) error {
	if !pruning {
		log.Info("refcount", ref.startRefCountHeight+offset, ref.dbCounter, ref.nodeCounter, len(ref.counts))
	} else {
		log.Info("pruning", ref.startPruningHeight+offset, ref.dbCounter, ref.nodeCounter, len(ref.counts))
	}

	return nil
}

func (ref *RefCounts) CreateRefCounts(hash common.Uint256) error {
	root, err := ref.trie.resolveHash(hash.ToArray(), true)
	if err != nil {
		return err
	}

	ref.trie.root = root
	ref.dbCounter = 0
	ref.nodeCounter = 0
	err = ref.createRefCounts(ref.trie.root)
	if err != nil {
		return err
	}

	return nil
}

func (ref *RefCounts) createRefCounts(n node) error {
	ref.nodeCounter++
	switch n := n.(type) {
	case *shortNode:
		hash, _ := n.cache()
		hs, _ := common.Uint256ParseFromBytes(hash)
		if count := ref.counts[hs]; count == 0 {
			if err := ref.createRefCounts(n.Val); err != nil {
				return err
			}
		}

		ref.counts[hs]++
		return nil
	case *fullNode:
		hash, _ := n.cache()
		hs, _ := common.Uint256ParseFromBytes(hash)
		if count := ref.counts[hs]; count == 0 {
			for i := 0; i < 17; i++ {
				if n.Children[i] != nil {
					err := ref.createRefCounts(n.Children[i])
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
		if count := ref.counts[hs]; count != 0 {
			ref.counts[hs]++
			return nil
		}
		child, err := ref.trie.resolveHash(n, true)
		if err != nil {
			return err
		}
		ref.dbCounter++
		return ref.createRefCounts(child)
	case nil:
		return nil
	case valueNode:
		return nil
	default:
		panic(fmt.Sprintf("invalid node type : %v, %v", reflect.TypeOf(n), n))

	}
}

func (ref *RefCounts) Prune(hash common.Uint256) error {
	root, err := ref.trie.resolveHash(hash.ToArray(), true)
	if err != nil {
		return err
	}
	ref.trie.root = root

	err = ref.prune(ref.trie.root)
	if err != nil {
		return err
	}

	return nil
}

func (ref *RefCounts) prune(n node) error {
	switch n := n.(type) {
	case *shortNode:
		hash, _ := n.cache()
		hs, _ := common.Uint256ParseFromBytes(hash)
		count, ok := ref.counts[hs]
		if !ok {
			panic(fmt.Sprintf("cannot get refcount, %v", hs.ToHexString()))
		}
		if count == 0 {
			panic(fmt.Sprintf("refCount cannot be zero, %v", hs.ToHexString()))
		}

		if count > 1 {
			ref.counts[hs] = count - 1
			return nil
		}

		delete(ref.counts, hs)
		ref.trie.db.BatchDelete(db.TrieNodeKey([]byte(hash)))
		return ref.prune(n.Val)

	case *fullNode:
		hash, _ := n.cache()
		hs, _ := common.Uint256ParseFromBytes(hash)
		count, ok := ref.counts[hs]
		if !ok {
			panic(fmt.Sprintf("cannot get refcount, %v", hs.ToHexString()))
		}
		if count == 0 {
			panic(fmt.Sprintf("refCount cannot be zero, %v", hs.ToHexString()))
		}

		if count > 1 {
			ref.counts[hs] = count - 1
			return nil
		}

		delete(ref.counts, hs)
		ref.trie.db.BatchDelete(db.TrieNodeKey([]byte(hash)))
		for i := 0; i < 17; i++ {
			if n.Children[i] != nil {
				if err := ref.prune(n.Children[i]); err != nil {
					return err
				}
			}
		}

		return nil
	case hashNode:
		hs, _ := common.Uint256ParseFromBytes([]byte(n))
		count, ok := ref.counts[hs]
		if !ok {
			panic(fmt.Sprintf("cannot get refcount, %v", hs.ToHexString()))
		}
		if count == 0 {
			panic(fmt.Sprintf("refCount cannot be zero, %v", hs.ToHexString()))
		}

		if count > 1 {
			ref.counts[hs] = count - 1
			return nil
		}

		child, err := ref.trie.resolveHash(n, true)
		if err != nil {
			return err
		}
		return ref.prune(child)
	case nil:
		return nil
	case valueNode:
		return nil
	default:
		panic(fmt.Sprintf("invalid node type : %v, %v", reflect.TypeOf(n), n))

	}
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

func (ref *RefCounts) PersistPruningHeights() error {
	heightBuffer := make([]byte, 4)
	binary.LittleEndian.PutUint32(heightBuffer[:], ref.targetRefCountHeight)
	err := ref.trie.db.BatchPut(db.TrieRefCountHeightKey(), heightBuffer)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(heightBuffer[:], ref.targetPruningHeight)
	err = ref.trie.db.BatchPut(db.TriePrunedHeightKey(), heightBuffer)
	if err != nil {
		return err
	}

	return nil
}

func (ref *RefCounts) Verify(hash common.Uint256) error {
	root, err := ref.trie.resolveHash(hash.ToArray(), false)
	if err != nil {
		return err
	}
	ref.trie.root = root

	err = ref.trie.traverse(ref.trie.root)
	if err != nil {
		panic(err)
	}

	log.Info("verification has done.")

	return nil
}
