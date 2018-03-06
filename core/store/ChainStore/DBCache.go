package ChainStore

import (
	"nkn-core/smartcontract/storage"
	"bytes"
	"nkn-core/smartcontract/states"
	"nkn-core/core/store"
	"math/big"
	"nkn-core/common"
)

type DBCache struct {
	RWSet *storage.RWSet
	db    *ChainStore
}

func NewDBCache(db *ChainStore) *DBCache {
	return &DBCache{
		RWSet: storage.NewRWSet(),
		db: db,
	}
}

func (cache *DBCache) Commit() (err error) {
	rwSet := cache.RWSet.WriteSet
	for k, v := range rwSet {
		key := make([]byte, 0)
		key = append([]byte{byte(v.Prefix)}, []byte(k)...)
		if v.IsDeleted {
			err = cache.db.st.BatchDelete(key)
		} else {
			b := new(bytes.Buffer)
			v.Item.Serialize(b)
			value := make([]byte, 0)
			value = append(value, b.Bytes()...)
			err = cache.db.st.BatchPut(key, value)
		}
	}
	return
}

func (cache *DBCache) TryGetInternal(prefix store.DataEntryPrefix, key string) (states.IStateValueInterface, error) {
	value, err := cache.db.st.Get(append([]byte{byte(prefix)}, []byte(key)...))
	if err != nil {
		return nil, err
	}
	return states.GetStateValue(prefix, value)
}

func (cache *DBCache) GetOrAdd(prefix store.DataEntryPrefix, key string, value states.IStateValueInterface) (states.IStateValueInterface, error) {
	if v, ok := cache.RWSet.WriteSet[key]; ok {
		if v.IsDeleted {
			v.Item = value
			v.IsDeleted = false
		}
	} else {
		item, err := cache.TryGetInternal(prefix, key)
		if err != nil && err.Error() != ErrDBNotFound.Error() {
			return nil, err
		}
		write := &storage.Write{
			Prefix: prefix,
			Item: item,
			Key: key,
			IsDeleted: false,
		}
		if write.Item == nil {
			write.Item = value

		}
		cache.RWSet.WriteSet[key] = write
	}
	return cache.RWSet.WriteSet[key].Item, nil
}

func (cache *DBCache) TryGet(prefix store.DataEntryPrefix, key string) (states.IStateValueInterface, error) {
	if v, ok := cache.RWSet.WriteSet[key]; ok {
		return v.Item, nil
	} else {
		return cache.TryGetInternal(prefix, key)
	}
}

func (cache *DBCache) GetWriteSet() *storage.RWSet {
	return cache.RWSet
}

func (cache *DBCache) GetState(codeHash common.Uint160, loc common.Hash) (common.Hash, error) {
	key := states.NewStorageKey(&codeHash, loc.Bytes())
	keyStr := storage.KeyToStr(key)
	value, err := cache.TryGet(store.ST_Storage, keyStr)
	if err != nil {
		return common.Hash{}, err
	}
	return common.BytesToHash(value.(*states.StorageItem).Value), nil
}

func (cache *DBCache) SetState(codeHash common.Uint160, loc, value common.Hash) {
	key := states.NewStorageKey(&codeHash, loc.Bytes())
	item := states.NewStorageItem(value.Bytes())
	keyStr := storage.KeyToStr(key)
	cache.RWSet.Add(store.ST_Storage, keyStr, item)
}

func (cache *DBCache) GetCode(codeHash common.Uint160) ([]byte, error) {
	skey := storage.KeyToStr(&codeHash)
	value, err := cache.TryGet(store.ST_Storage, skey)
	if err != nil {
		return nil, err
	}
	return value.(*states.StorageItem).Value, nil
}

func (cache *DBCache) SetCode(codeHash common.Uint160, code []byte) {
	item := states.NewStorageItem(code)
	skey := storage.KeyToStr(&codeHash)
	cache.RWSet.Add(store.ST_Storage, skey, item)
}

func (cache *DBCache) GetBalance(common.Uint160) *big.Int {
	return big.NewInt(100)
}

func (cache *DBCache) GetCodeSize(common.Uint160) int {
	return 0
}

func (cache *DBCache) AddBalance(common.Uint160, *big.Int) {
}

func (cache *DBCache) Suicide(codeHash common.Uint160) bool {
	skey := storage.KeyToStr(&codeHash)
	cache.RWSet.Delete(skey)
	return true
}





