package db

import (
	"encoding/json"
	"io/ioutil"
	"math/rand"

	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/util/log"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type IStore interface {
	Put(key []byte, value []byte) error
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)
	Delete(key []byte) error
	NewBatch() error
	BatchPut(key []byte, value []byte) error
	BatchDelete(key []byte) error
	BatchCommit() error
	Compact() error
	Close() error
	NewIterator(prefix []byte) IIterator
}

type LevelDBStore struct {
	db    *leveldb.DB // LevelDB instance
	batch *leveldb.Batch
}

type LevelDBConfig struct {
	CompactionL0Trigger    int
	WriteL0SlowdownTrigger int
	WriteL0PauseTrigger    int
	CompactionTableSize    int
	WriteBuffer            int
	BlockCacheCapacity     int
	DisableSeeksCompaction bool
}

const (
	// used to compute the size of bloom filter bits array, too small will lead to
	// high false positive rate.
	BITSPERKEY     = 10
	DBConfigSuffix = ".config"
)

func NewLevelDBConfig(file string) *LevelDBConfig {
	c := &LevelDBConfig{}
	if b, err := ioutil.ReadFile(file); err == nil {
		if err = json.Unmarshal(b, c); err == nil {
			return c
		}
	}

	triggerRandFactor := 1 + rand.Float32()
	sizeRandFactor := 1 + rand.Float32()

	compactionL0Trigger := int(4 * triggerRandFactor)
	compactionTableSize := int(2 * opt.MiB * sizeRandFactor)

	c = &LevelDBConfig{
		CompactionL0Trigger:    compactionL0Trigger,
		WriteL0SlowdownTrigger: 2 * compactionL0Trigger,
		WriteL0PauseTrigger:    3 * compactionL0Trigger,
		CompactionTableSize:    compactionTableSize,
		WriteBuffer:            2 * compactionTableSize,
		BlockCacheCapacity:     4 * compactionTableSize,
		DisableSeeksCompaction: true,
	}

	b, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		log.Errorf("Marshal leveldb opts error: %v", err)
		return c
	}

	err = ioutil.WriteFile(file, b, 0666)
	if err != nil {
		log.Errorf("Save leveldb opts error: %v", err)
		return c
	}

	return c
}

func NewLevelDBStore(file string) (*LevelDBStore, error) {
	c := NewLevelDBConfig(file + DBConfigSuffix)
	opts := &opt.Options{
		CompactionL0Trigger:    c.CompactionL0Trigger,
		WriteL0SlowdownTrigger: c.WriteL0SlowdownTrigger,
		WriteL0PauseTrigger:    c.WriteL0PauseTrigger,
		CompactionTableSize:    c.CompactionTableSize,
		WriteBuffer:            c.WriteBuffer,
		BlockCacheCapacity:     c.BlockCacheCapacity,
		DisableSeeksCompaction: c.DisableSeeksCompaction,
		OpenFilesCacheCapacity: int(config.Parameters.DBFilesCacheCapacity),
		Filter:                 filter.NewBloomFilter(BITSPERKEY),
	}

	db, err := leveldb.OpenFile(file, opts)
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

func (self *LevelDBStore) Has(key []byte) (bool, error) {
	return self.db.Has(key, nil)
}

func (self *LevelDBStore) Delete(key []byte) error {
	return self.db.Delete(key, nil)
}

func (self *LevelDBStore) NewBatch() error {
	self.batch = new(leveldb.Batch)
	return nil
}

func (self *LevelDBStore) BatchPut(key []byte, value []byte) error {
	self.batch.Put(key, value)
	return nil
}

func (self *LevelDBStore) BatchDelete(key []byte) error {
	self.batch.Delete(key)
	return nil
}

func (self *LevelDBStore) BatchCommit() error {
	err := self.db.Write(self.batch, nil)
	if err != nil {
		return err
	}
	return nil
}

func (self *LevelDBStore) Compact() error {
	r := util.Range{
		Start: nil,
		Limit: nil,
	}
	return self.db.CompactRange(r)
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
