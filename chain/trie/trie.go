package trie

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/nknorg/nkn/chain/db"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/log"
)

type Database interface {
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)
	Delete(key []byte) error
	Compact() error
	NewBatch() error
	BatchPut(key, value []byte) error
	BatchDelete(key []byte) error
	BatchCommit() error
	NewIterator(prefix []byte) db.IIterator
}

type Trie struct {
	root         node
	db           Database
	originalRoot common.Uint256
}

func New(hash common.Uint256, db Database) (*Trie, error) {
	trie := &Trie{db: db, originalRoot: hash}
	if hash != common.EmptyUint256 && db != nil {
		root, err := trie.resolveHash(hash.ToArray(), false)
		if err != nil {
			return nil, err
		}
		trie.root = root
	}
	return trie, nil
}

func (t *Trie) NodeIterator(start []byte) NodeIterator {
	return newNodeIterator(t, start)
}

func (t *Trie) Get(key []byte) []byte {
	res, err := t.TryGet(key)
	if err != nil {
		log.Errorf("Unhandled trie error: %v", err)
		return nil
	}
	return res
}

func (t *Trie) TryGet(key []byte) ([]byte, error) {
	key = keyBytesToHex(key)
	value, newRoot, err := t.tryGet(t.root, key, 0)
	if err != nil {
		return nil, err
	}
	t.root = newRoot
	return value, nil
}

func (t *Trie) tryGet(origNode node, key []byte, pos int) (value []byte, newNode node, err error) {
	switch n := origNode.(type) {
	case nil:
		return nil, nil, nil
	case valueNode:
		return n, n, nil
	case *shortNode:
		if len(key)-pos < len(n.Key) || !bytes.Equal(n.Key, key[pos:pos+len(n.Key)]) {
			return nil, n, nil
		}
		value, newNode, err = t.tryGet(n.Val, key, pos+len(n.Key))
		if err != nil {
			return nil, n, err
		}
		n = n.copy()
		n.Val = newNode
		return value, n, nil
	case *fullNode:
		value, newNode, err = t.tryGet(n.Children[key[pos]], key, pos+1)
		if err != nil {
			return nil, n, err
		}
		n = n.copy()
		n.Children[key[pos]] = newNode
		return value, n, nil
	case hashNode:
		child, err := t.resolveHash(n, false)
		if err != nil {
			return nil, n, err
		}
		return t.tryGet(child, key, pos)
	default:
		log.Fatalf("Invalid find node type: %v", origNode)
	}
	return nil, nil, nil
}

func (t *Trie) Update(key, value []byte) {
	if err := t.TryUpdate(key, value); err != nil {
		log.Errorf("Unhandled trie error: %v", err)
	}
}

func (t *Trie) TryUpdate(key, value []byte) error {
	k := keyBytesToHex(key)
	n, err := t.insert(t.root, k, valueNode(value))
	if err != nil {
		return err
	}
	t.root = n
	return nil
}

func (t *Trie) insert(n node, key []byte, value node) (node, error) {
	if len(key) == 0 {
		return value, nil
	}
	switch n := n.(type) {
	case *shortNode:
		matchLen := prefixLen(key, n.Key)
		if matchLen == len(n.Key) {
			nn, err := t.insert(n.Val, key[matchLen:], value)
			if err != nil {
				return nil, err
			}
			return &shortNode{Key: n.Key, Val: nn}, nil
		}
		branch := &fullNode{flags: nodeFlag{}}
		var err error

		branch.Children[n.Key[matchLen]], err = t.insert(nil, n.Key[matchLen+1:], n.Val)
		if err != nil {
			return nil, err
		}
		branch.Children[key[matchLen]], err = t.insert(nil, key[matchLen+1:], value)
		if err != nil {
			return nil, err
		}
		if matchLen == 0 {
			return branch, nil
		}
		return &shortNode{key[:matchLen], branch, nodeFlag{dirty: true}}, nil
	case *fullNode:
		x, ok := n.Children[key[0]].(valueNode)
		if ok {
			log.Info("---------->", key[0], reflect.TypeOf(n.Children[key[0]]), len(x), n.fString(""))
		}
		nn, err := t.insert(n.Children[key[0]], key[1:], value)
		if err != nil {
			return nil, err
		}
		n = n.copy()
		n.Children[key[0]] = nn
		n.flags = nodeFlag{dirty: true}
		return n, nil
	case nil:
		return &shortNode{Key: key, Val: value, flags: nodeFlag{dirty: true}}, nil
	case hashNode:
		rn, err := t.resolveHash(n, false)
		if err != nil {
			return nil, err
		}
		nn, err := t.insert(rn, key, value)
		if err != nil {
			return rn, err
		}
		return nn, nil
	default:
		log.Fatalf("Invalid insert node type : %v,, %v", reflect.TypeOf(n), n)
	}
	return nil, nil
}

func (t *Trie) Delete(key []byte) {
	if err := t.TryDelete(key); err != nil {
		log.Errorf("Unhandled trie error: %v", err)
	}
}

func (t *Trie) TryDelete(key []byte) error {
	k := keyBytesToHex(key)
	n, err := t.delete(t.root, k)
	if err != nil {
		return err
	}
	t.root = n
	return nil
}

func (t *Trie) delete(n node, key []byte) (node, error) {
	switch n := n.(type) {
	case *shortNode:
		matchLen := prefixLen(key, n.Key)
		if matchLen < len(n.Key) {
			return n, nil
		}
		if matchLen == len(key) {
			return nil, nil
		}
		child, err := t.delete(n.Val, key[len(n.Key):])
		if err != nil {
			return n, err
		}
		switch child := child.(type) {
		case *shortNode:
			return &shortNode{Key: concat(n.Key, child.Key...), Val: child.Val}, nil
		default:
			return &shortNode{Key: n.Key, Val: child}, nil
		}
	case *fullNode:
		nn, err := t.delete(n.Children[key[0]], key[1:])
		if err != nil {
			return n, err
		}
		n = n.copy()
		n.flags = nodeFlag{}
		n.Children[key[0]] = nn
		pos := -1
		for i, ch := range n.Children {
			if ch != nil {
				if pos == -1 {
					pos = i
				} else {
					pos = -2
					break
				}
			}
		}
		if pos >= 0 {
			if pos != 16 {
				cNode, err := t.resolve(n.Children[pos])
				if err != nil {
					return nil, err
				}
				if c, ok := cNode.(*shortNode); ok {
					k := append([]byte{byte(pos)}, c.Key...)
					return &shortNode{Key: k, Val: c.Val}, nil
				}
			}
			return &shortNode{Key: []byte{byte(pos)}, Val: n.Children[pos]}, nil
		}
		return n, nil
	case valueNode:
		return nil, nil
	case nil:
		return nil, nil
	case hashNode:
		rn, err := t.resolveHash(n, false)
		if err != nil {
			return nil, err
		}
		nn, err := t.delete(rn, key)
		if err != nil {
			return rn, err
		}
		return nn, nil
	default:
		log.Fatalf("Invalid Delete Node Type: %v", n)
	}
	return nil, nil
}

func (t *Trie) Commit() (common.Uint256, error) {
	if t.db == nil {
		log.Fatal("Commit data to trie whit nil database")
	}
	return t.CommitTo(t.db)
}

func (t *Trie) CommitTo(db Database) (common.Uint256, error) {
	hash, cached, err := t.hashRoot(db)
	if err != nil {
		return common.Uint256{}, err
	}
	if hash == nil {
		return common.Uint256{}, nil
	}
	t.root = cached
	u256, err := common.Uint256ParseFromBytes(hash.(hashNode))
	if err != nil {
		return common.Uint256{}, err
	}
	return u256, nil
}

func (t *Trie) Hash() common.Uint256 {
	hash, cached, _ := t.hashRoot(nil)
	t.root = cached
	u, _ := common.Uint256ParseFromBytes(hash.(hashNode))
	return u
}

func (t *Trie) hashRoot(db Database) (node, node, error) {
	if t.root == nil {
		return nil, nil, nil
	}
	h := newHasher()
	defer returnHasherToPool(h)
	return h.hash(t.root, db, true)
}

func (t *Trie) resolve(n node) (node, error) {
	if n, ok := n.(hashNode); ok {
		return t.resolveHash(n, false)
	}
	return n, nil
}

func (t *Trie) resolveHash(n hashNode, needFlags bool) (node, error) {
	enc, err := t.db.Get(db.TrieNodeKey([]byte(n)))
	if err != nil {
		return nil, err
	}
	dec := mustDecodeNode(n, enc, needFlags)
	return dec, nil
}

func concat(s1 []byte, s2 ...byte) []byte {
	r := make([]byte, len(s1)+len(s2))
	copy(r, s1)
	copy(r[len(s1):], s2)
	return r
}

func (t *Trie) Copy() *Trie {
	cpy := *t
	return &cpy
}

func (t *Trie) TryTraverse() error {
	return t.traverse(t.root, true)
}

func (t *Trie) traverse(n node, needPrint bool) error {
	switch n := n.(type) {
	case *shortNode:
		if needPrint {
			hash, _ := n.cache()
			hs, _ := common.Uint256ParseFromBytes(hash)
			fmt.Println(hs.ToHexString())
		}
		if err := t.traverse(n.Val, needPrint); err != nil {
			return err
		}
		return nil
	case *fullNode:
		if needPrint {
			hash, _ := n.cache()
			hs, _ := common.Uint256ParseFromBytes(hash)
			fmt.Println(hs.ToHexString())
		}
		for i := 0; i < LenOfChildrenNodes; i++ {
			if n.Children[i] != nil {
				err := t.traverse(n.Children[i], needPrint)
				if err != nil {
					return err
				}
			}
		}
		return nil
	case hashNode:
		if needPrint {
			hs, _ := common.Uint256ParseFromBytes(n)
			fmt.Println(hs.ToHexString())
		}
		child, err := t.resolveHash(n, needPrint)
		if err != nil {
			return err
		}
		return t.traverse(child, needPrint)
	case nil:
		if needPrint {
			fmt.Println("<nil>")
		}
		return nil
	case valueNode:
		if needPrint {
			hs, _ := common.Uint256ParseFromBytes(n)
			fmt.Println(hs.ToHexString())
		}
		return nil
	default:
		log.Fatalf("Invalid node type : %v, %v", reflect.TypeOf(n), n)
	}
	return nil
}

func (t *Trie) NewRefCounts(targetRefCountHeight, targetPruningHeight uint32) (*RefCounts, error) {
	return NewRefCounts(t, targetRefCountHeight, targetPruningHeight)
}
