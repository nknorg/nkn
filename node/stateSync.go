package node

import (
	"github.com/nknorg/nkn/v2/config"
	"math"
	"time"

	"github.com/nknorg/nkn/v2/chain/db"
	"github.com/nknorg/nkn/v2/chain/trie"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/util/log"
)

const (
	fastSyncMaxBatchSize    = 100 * 1024
	fastSyncMaxStateReqSize = 500
)

type stateSync struct {
	root   common.Uint256 // State root currently being synced
	sched  *trie.Sync     // State trie sync scheduler defining the tasks
	hasher func([]byte) []byte

	done chan struct{} // Channel to signal termination completion

	hashesCh chan []common.Uint256
	stateCh  chan [][]byte // Response states channel

	peers []*RemoteNode // Set of active peers from which download can proceed
	db    db.IStore     // Database to state sync into

	bytesUncommitted int
}

// newStateSync creates a new state trie download scheduler. This method does not
// yet start the sync. The user needs to call run to initiate.
func newStateSync(db db.IStore, root common.Uint256, peers []*RemoteNode) *stateSync {
	return &stateSync{
		db:       db,
		root:     root,
		sched:    trie.NewSync(root),
		hasher:   trie.Sha256Key,
		done:     make(chan struct{}),
		stateCh:  make(chan [][]byte),
		hashesCh: make(chan []common.Uint256),
		peers:    peers,
	}
}

// loop is the main event loop of a state trie sync. It is responsible for the
// assignment of new tasks to peers (including sending it to them) as well as
// for the processing of inbound data.
func (s *stateSync) loop() error {
	go s.startWorkerThread()

	go func() {
		for {
			log.Info("Pending states:", s.sched.Pending())
			select {
			case <-s.done:
				log.Info("Done with pending states:", s.sched.Pending())
				return
			case <-time.After(3 * time.Second):
			}
		}
	}()

	go func() {
		for {
			select {
			case <-s.sched.DataAvailable:
			case <-s.done:
				return
			}
			for {
				jobs, _ := s.sched.Pop(fastSyncMaxStateReqSize)
				if len(jobs) == 0 {
					break
				}
				s.hashesCh <- jobs
			}
		}
	}()

	for s.sched.Pending() > 0 {
		if err := s.commit(false); err != nil {
			return err
		}
		select {
		case data := <-s.stateCh:
			for _, blob := range data {
				_, err := s.processNodeData(blob)
				if err != nil {
					return err
				}
				s.bytesUncommitted += len(blob)
			}
		case <-s.done:
			return nil
		}
	}
	err := s.commit(true)
	if err != nil {
		return err
	}
	return nil
}

func (s *stateSync) commit(force bool) error {
	if !force && s.bytesUncommitted < fastSyncMaxBatchSize {
		return nil
	}
	s.db.NewBatch()
	if err := s.sched.Commit(s.db); err != nil {
		return err
	}
	if err := s.db.BatchCommit(); err != nil {
		return err
	}
	s.bytesUncommitted = 0
	return nil
}

// processNodeData tries to inject a trie node data blob delivered from a remote
// peer into the state trie, returning whether anything useful was written or any
// error occurred.
func (s *stateSync) processNodeData(blob []byte) (common.Uint256, error) {
	res := trie.SyncResult{Data: blob}
	var err error
	res.Hash, err = common.Uint256ParseFromBytes(s.hasher(blob))
	if err != nil {
		return common.EmptyUint256, err
	}
	err = s.sched.Process(res)
	return res.Hash, err
}

func (s *stateSync) startWorker(p *RemoteNode) {
	var hashes []common.Uint256
	fail := 0
	for {
		select {
		case hashes = <-s.hashesCh:
		case <-s.done:
			return
		}

		resp, err := p.GetStates(hashes)

		if err != nil {
			s.sched.Push(hashes, math.MaxInt64)
			fail++
			if fail >= maxSyncWorkerFails {
				return
			}
			continue
		}
		if len(resp) > 0 {
			s.stateCh <- resp
		}
	}
}

// run starts the task assignment and response processing loop, blocking until
// it finishes, and finally notifying any goroutines waiting for the loop to
// finish.
func (s *stateSync) run() error {
	err := s.loop()
	close(s.done)
	return err
}

func (s *stateSync) startWorkerThread() {
	var workerId uint32
	peersNum := uint32(len(s.peers))
	workerNum := peersNum * concurrentSyncRequestPerNeighbor
	if workerNum > config.Parameters.SyncStateMaxThread {
		workerNum = config.Parameters.SyncStateMaxThread
	}
	for workerId = 0; workerId < workerNum; workerId++ {
		go func(workerId uint32) {
			s.startWorker(s.peers[workerId%peersNum])
		}(workerId)
	}
}
