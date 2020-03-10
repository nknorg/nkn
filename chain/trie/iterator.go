package trie

import (
	"bytes"
	"errors"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/log"
)

type Iterator struct {
	nodeIt NodeIterator

	Key   []byte
	Value []byte
	Err   error
}

func NewIterator(it NodeIterator) *Iterator {
	return &Iterator{
		nodeIt: it,
	}
}

func (it *Iterator) Next() bool {
	for it.nodeIt.Next(true) {
		if it.nodeIt.Leaf() {
			it.Key = it.nodeIt.LeafKey()
			it.Value = it.nodeIt.LeafBlob()
			return true
		}
	}
	it.Key = nil
	it.Value = nil
	it.Err = it.nodeIt.Error()
	return false
}

type NodeIterator interface {
	Next(bool) bool
	Error() error
	Hash() common.Uint256
	Parent() common.Uint256
	Path() []byte
	Leaf() bool
	LeafKey() []byte
	LeafBlob() []byte
}

type nodeIteratorState struct {
	hash    common.Uint256
	node    node
	parent  common.Uint256
	index   int
	pathlen int
}

type nodeIterator struct {
	trie  *Trie
	stack []*nodeIteratorState
	path  []byte
	err   error
}

var errIteratorEnd = errors.New("end of iteration")

type seekError struct {
	key []byte
	err error
}

func (e seekError) Error() string {
	return "seek error: " + e.err.Error()
}

func newNodeIterator(trie *Trie, start []byte) NodeIterator {
	if trie.Hash() == common.EmptyUint256 {
		return new(nodeIterator)
	}
	it := &nodeIterator{trie: trie}
	it.err = it.seek(start)
	return it
}

func (it *nodeIterator) Hash() common.Uint256 {
	if len(it.stack) == 0 {
		return common.EmptyUint256
	}
	return it.stack[len(it.stack)-1].hash
}

func (it *nodeIterator) Parent() common.Uint256 {
	if len(it.stack) == 0 {
		return common.EmptyUint256
	}
	return it.stack[len(it.stack)-1].parent
}

func (it *nodeIterator) Leaf() bool {
	return hasTerm(it.path)
}

func (it *nodeIterator) LeafKey() []byte {
	if len(it.stack) > 0 {
		if _, ok := it.stack[len(it.stack)-1].node.(valueNode); ok {
			return hexToKeyBytes(it.path)
		}
	}
	log.Fatal("Node iterator not at leaf")
	return nil
}

func (it *nodeIterator) LeafBlob() []byte {
	if len(it.stack) > 0 {
		if node, ok := it.stack[len(it.stack)-1].node.(valueNode); ok {
			return []byte(node)
		}
	}
	log.Fatal("Node iterator not at leaf")
	return nil
}

func (it *nodeIterator) Path() []byte {
	return it.path
}

func (it *nodeIterator) Error() error {
	if it.err == errIteratorEnd {
		return nil
	}
	if seek, ok := it.err.(seekError); ok {
		return seek.err
	}
	return it.err
}

func (it *nodeIterator) Next(descend bool) bool {
	if it.err == errIteratorEnd {
		return false
	}
	if seek, ok := it.err.(seekError); ok {
		if it.err = it.seek(seek.key); it.err != nil {
			return false
		}
	}
	// Otherwise step forward with the iterator and report any errors.
	state, parentIndex, path, err := it.peek(descend)
	it.err = err
	if it.err != nil {
		return false
	}
	it.push(state, parentIndex, path)
	return true
}

func (it *nodeIterator) seek(prefix []byte) error {
	// The path we're looking for is the hex encoded key without terminator.
	key := keyBytesToHex(prefix)
	key = key[:len(key)-1]
	// Move forward until we're just before the closest match to key.
	for {
		state, parentIndex, path, err := it.peek(bytes.HasPrefix(key, it.path))
		if err == errIteratorEnd {
			return errIteratorEnd
		} else if err != nil {
			return seekError{prefix, err}
		} else if bytes.Compare(path, key) >= 0 {
			return nil
		}
		it.push(state, parentIndex, path)
	}
}

func (it *nodeIterator) peek(descend bool) (*nodeIteratorState, *int, []byte, error) {
	if len(it.stack) == 0 {
		// Initialize the iterator if we've just started.
		root := it.trie.Hash()
		state := &nodeIteratorState{node: it.trie.root, index: -1}
		if root != common.EmptyUint256 {
			state.hash = root
		}
		err := state.resolve(it.trie)
		return state, nil, nil, err
	}
	if !descend {
		// If we're skipping children, pop the current node first
		it.pop()
	}

	// Continue iteration to the next child
	for len(it.stack) > 0 {
		parent := it.stack[len(it.stack)-1]
		ancestor := parent.hash
		if ancestor == common.EmptyUint256 {
			ancestor = parent.parent
		}
		state, path, ok := it.nextChild(parent, ancestor)
		if ok {
			if err := state.resolve(it.trie); err != nil {
				return parent, &parent.index, path, err
			}
			return state, &parent.index, path, nil
		}
		// No more child nodes, move back up.
		it.pop()
	}
	return nil, nil, nil, errIteratorEnd
}

func (st *nodeIteratorState) resolve(tr *Trie) error {
	if hashNode, ok := st.node.(hashNode); ok {
		resolved, err := tr.resolveHash(hashNode, false)
		if err != nil {
			return err
		}
		st.node = resolved
		st.hash, _ = common.Uint256ParseFromBytes(hashNode)
	}
	return nil
}

func (it *nodeIterator) nextChild(parent *nodeIteratorState, ancestor common.Uint256) (*nodeIteratorState, []byte, bool) {
	switch node := parent.node.(type) {
	case *fullNode:
		// Full node, move to the first non-nil child.
		for i := parent.index + 1; i < len(node.Children); i++ {
			child := node.Children[i]
			if child != nil {
				hashNode, _ := child.cache()
				hash, _ := common.Uint256ParseFromBytes(hashNode)
				state := &nodeIteratorState{
					hash:    hash,
					node:    child,
					parent:  ancestor,
					index:   -1,
					pathlen: len(it.path),
				}
				path := append(it.path, byte(i))
				parent.index = i - 1
				return state, path, true
			}
		}
	case *shortNode:
		// Short node, return the pointer singleton child
		if parent.index < 0 {
			hashNode, _ := node.Val.cache()
			hash, _ := common.Uint256ParseFromBytes(hashNode)
			state := &nodeIteratorState{
				hash:    hash,
				node:    node.Val,
				parent:  ancestor,
				index:   -1,
				pathlen: len(it.path),
			}
			path := append(it.path, node.Key...)
			return state, path, true
		}
	}
	return parent, it.path, false
}

func (it *nodeIterator) push(state *nodeIteratorState, parentIndex *int, path []byte) {
	it.path = path
	it.stack = append(it.stack, state)
	if parentIndex != nil {
		*parentIndex++
	}
}

func (it *nodeIterator) pop() {
	parent := it.stack[len(it.stack)-1]
	it.path = it.path[:parent.pathlen]
	it.stack = it.stack[:len(it.stack)-1]
}
