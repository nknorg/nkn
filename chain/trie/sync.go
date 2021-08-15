package trie

import (
	"sync"

	"github.com/nknorg/nkn/v2/chain/db"
	"github.com/nknorg/nkn/v2/util/log"

	"github.com/nknorg/nkn/v2/common"
)

type SyncPath []byte

// request represents a scheduled or already in-flight state retrieval request.
type request struct {
	path []byte         // Merkle path leading to this node for prioritization
	data []byte         // Data content of the node, cached until all subtrees complete
	hash common.Uint256 // Hash of the node data content to retrieve
}

// SyncResult is a response with requested data along with it's hash.
type SyncResult struct {
	Hash common.Uint256 // Hash of the originally unknown trie node
	Data []byte         // Data content of the retrieved node
}

// Sync is the main state trie synchronisation scheduler, which provides yet
// unknown trie hashes to retrieve, accepts node data associated with said hashes
// and reconstructs the trie step by step until all is done.
type Sync struct {
	sync.RWMutex
	membatch map[common.Uint256][]byte   // Memory buffer to avoid frequent database writes
	nodeReqs map[common.Uint256]*request // Pending requests pertaining to a trie node hash
	queue    *common.PriorityQueue       // Priority queue with the pending requests

	DataAvailable chan struct{} // Notify when new states arrived
}

func NewSync(root common.Uint256) *Sync {
	ts := &Sync{
		queue:         common.NewPrque(),
		membatch:      make(map[common.Uint256][]byte),
		nodeReqs:      make(map[common.Uint256]*request),
		DataAvailable: make(chan struct{}, 1),
	}

	ts.AddSubTrie(root, nil)
	return ts
}

// AddSubTrie registers a new trie to the sync code, rooted at the designated parent.
func (s *Sync) AddSubTrie(root common.Uint256, path []byte) {
	s.Lock()
	defer s.Unlock()
	req := &request{
		path: path,
		hash: root,
	}
	s.schedule(req)
}

// Pending returns the number of state entries currently pending for download.
func (s *Sync) Pending() int {
	s.RLock()
	defer s.RUnlock()
	return len(s.nodeReqs)
}

// Pop return the specific number of hash jobs from heap
func (s *Sync) Pop(max int) ([]common.Uint256, []int64) {
	s.Lock()
	defer s.Unlock()
	var (
		nodeHashes []common.Uint256
		prios      []int64
	)

	for !s.queue.Empty() && (max == 0 || len(nodeHashes) < max) {
		item, p := s.queue.Pop()
		hash := item.(common.Uint256)
		nodeHashes = append(nodeHashes, hash)
		prios = append(prios, p)
	}
	return nodeHashes, prios
}

func (s *Sync) Push(hashes []common.Uint256, prio int64) {
	s.Lock()
	defer s.Unlock()
	for _, h := range hashes {
		s.queue.Push(h, prio)
	}
	select {
	case s.DataAvailable <- struct{}{}:
	default:
	}
}

// Commit flushes the data stored in the internal membatch out to persistent
// storage, returning any occurred error.
func (s *Sync) Commit(sdb db.IStore) error {
	s.Lock()
	defer s.Unlock()
	// Dump the membatch into a database sdb
	for key, value := range s.membatch {
		err := sdb.BatchPut(db.TrieNodeKey(key.ToArray()), value)
		if err != nil {
			return err
		}
	}

	// Drop the membatch data and return
	s.membatch = make(map[common.Uint256][]byte)
	return nil
}

// Process injects the received data for requested item.
func (s *Sync) Process(result SyncResult) error {
	s.Lock()
	defer s.Unlock()
	// There is an pending node request for this data, fill it.
	if req := s.nodeReqs[result.Hash]; req != nil && req.data == nil {
		// Decode the node data content and update the request
		node, err := decodeNode(result.Hash[:], result.Data, false)
		if err != nil {
			return err
		}
		req.data = result.Data

		// Create and schedule a request for all the children nodes
		requests, err := s.children(req, node)
		if err != nil {
			return err
		}
		if len(requests) > 0 {
			for _, child := range requests {
				s.schedule(child)
			}
		}
		s.commit(req)
	}
	return nil
}

// schedule inserts a new state retrieval request into the fetch queue
func (s *Sync) schedule(req *request) {
	s.nodeReqs[req.hash] = req

	prio := int64(len(req.path)) << 56
	for i := 0; i < 14 && i < len(req.path); i++ {
		prio |= int64(15-req.path[i]) << (52 - i*4) // 15-nibble => lexicographic order
	}
	s.queue.Push(req.hash, prio)
	select {
	case s.DataAvailable <- struct{}{}:
	default:
	}
}

// children retrieves all the missing children of a state trie entry for future
// retrieval scheduling.
func (s *Sync) children(req *request, object node) ([]*request, error) {
	type child struct {
		path []byte
		node node
	}
	var children []child

	switch n := (object).(type) {
	case *shortNode:
		key := n.Key
		key = key[:len(key)-1]

		children = []child{{
			node: n.Val,
			path: append(append([]byte(nil), req.path...), key...),
		}}
	case *fullNode:
		for i := 0; i < 17; i++ {
			if n.Children[i] != nil {
				children = append(children, child{
					node: n.Children[i],
					path: append(append([]byte(nil), req.path...), byte(i)),
				})
			}
		}
	default:
		log.Fatalf("unknown node: %+v", n)
	}
	requests := make([]*request, 0, len(children))
	for _, child := range children {
		// If the child references another node, resolve or schedule
		if node, ok := (child.node).(hashNode); ok {
			hash, err := common.Uint256ParseFromBytes(node)
			if err != nil {
				return requests, err
			}
			if _, ok := s.membatch[hash]; ok {
				continue
			}
			r := &request{
				path: req.path,
				hash: hash,
			}
			requests = append(requests, r)
		}
	}
	return requests, nil
}

// commit finalizes a retrieval request and stores it into the membatch
func (s *Sync) commit(req *request) {
	s.membatch[req.hash] = req.data
	delete(s.nodeReqs, req.hash)
}
