package db

import (
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type IStore interface {
	Put(key []byte, value []byte) error
	Get(key []byte) ([]byte, error)
	Delete(key []byte) error
	NewBatch() error
	NewSepBatch() IBatch
	BatchPut(key []byte, value []byte) error
	BatchDelete(key []byte) error
	BatchCommit() error
	Close() error
	NewIterator(prefix []byte) IIterator
}

type IBatch interface {
	BatchPut(key []byte, value []byte) error
	BatchDelete(key []byte) error
	BatchCommit() error
}

type LevelDBStore struct {
	db    *leveldb.DB // LevelDB instance
	batch *leveldb.Batch
	mu    sync.RWMutex
}

// used to compute the size of bloom filter bits array .
// too small will lead to high false positive rate.
const BITSPERKEY = 10

func NewLevelDBStore(file string) (*LevelDBStore, error) {

	// default Options
	o := opt.Options{
		NoSync: false,
		Filter: filter.NewBloomFilter(BITSPERKEY),
	}

	db, err := leveldb.OpenFile(file, &o)

	if _, corrupted := err.(*errors.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(file, nil)
	}

	if err != nil {
		return nil, err
	}

	return &LevelDBStore{
		db:    db,
		batch: nil,
	}, nil
}

func (self *LevelDBStore) Put(key []byte, value []byte) error {
	return self.db.Put(key, value, nil)
}

func (self *LevelDBStore) Get(key []byte) ([]byte, error) {
	dat, err := self.db.Get(key, nil)
	return dat, err
}

func (self *LevelDBStore) Delete(key []byte) error {
	return self.db.Delete(key, nil)
}

func (self *LevelDBStore) NewBatch() error {
	self.batch = new(leveldb.Batch)
	return nil
}

func (self *LevelDBStore) NewSepBatch() IBatch {
	return &LevelDBStore{
		db:    self.db,
		batch: new(leveldb.Batch),
	}
}

func (self *LevelDBStore) BatchPut(key []byte, value []byte) error {
	self.mu.Lock()
	self.batch.Put(key, value)
	self.mu.Unlock()
	return nil
}

func (self *LevelDBStore) BatchDelete(key []byte) error {
	self.mu.Lock()
	self.batch.Delete(key)
	self.mu.Unlock()
	return nil
}

func (self *LevelDBStore) BatchCommit() error {
	self.mu.Lock()
	err := self.db.Write(self.batch, nil)
	self.mu.Unlock()

	return err
}

func (self *LevelDBStore) Close() error {
	err := self.db.Close()
	return err
}

func (self *LevelDBStore) NewIterator(prefix []byte) IIterator {

	iter := self.db.NewIterator(util.BytesPrefix(prefix), nil)

	return &Iterator{
		iter: iter,
	}
}
