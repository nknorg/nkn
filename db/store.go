package db

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	. "github.com/nknorg/nkn/core/asset"
	"github.com/nknorg/nkn/core/contract/program"
	. "github.com/nknorg/nkn/core/ledger"
	tx "github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/log"
)

const (
	HeaderHashListCount = 2000
	CleanCacheThreshold = 2
	TaskChanCap         = 4
)

var (
	ErrDBNotFound = errors.New("leveldb: not found")
)

type persistTask interface{}
type persistHeaderTask struct {
	header *Header
}
type persistBlockTask struct {
	block  *Block
	ledger *Ledger
}

type ChainStore struct {
	st IStore

	taskCh chan persistTask
	quit   chan chan bool

	mu          sync.RWMutex // guard the following var
	headerIndex map[uint32]Uint256
	blockCache  map[Uint256]*Block
	headerCache map[Uint256]*Header

	currentBlockHeight uint32
	storedHeaderCount  uint32
}

func NewStore(file string) (IStore, error) {
	ldbs, err := NewLevelDBStore(file)

	return ldbs, err
}

func NewLedgerStore() (ILedgerStore, error) {
	// TODO: read config file decide which db to use.
	cs, err := NewChainStore("Chain")
	if err != nil {
		return nil, err
	}

	return cs, nil
}

func NewChainStore(file string) (*ChainStore, error) {

	st, err := NewStore(file)
	if err != nil {
		return nil, err
	}

	chain := &ChainStore{
		st:                 st,
		headerIndex:        map[uint32]Uint256{},
		blockCache:         map[Uint256]*Block{},
		headerCache:        map[Uint256]*Header{},
		currentBlockHeight: 0,
		storedHeaderCount:  0,
		taskCh:             make(chan persistTask, TaskChanCap),
		quit:               make(chan chan bool, 1),
	}

	go chain.loop()

	return chain, nil
}

func (cs *ChainStore) Close() {
	closed := make(chan bool)
	cs.quit <- closed
	<-closed

	cs.st.Close()
}

func (cs *ChainStore) loop() {
	for {
		select {
		case t := <-cs.taskCh:
			switch task := t.(type) {
			case *persistHeaderTask:
				cs.handlePersistHeaderTask(task.header)
			case *persistBlockTask:
				cs.handlePersistBlockTask(task.block, task.ledger)
			}

		case closed := <-cs.quit:
			closed <- true
			return
		}
	}
}

// can only be invoked by backend write goroutine
func (cs *ChainStore) clearCache() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	currBlockHeight := cs.currentBlockHeight
	for hash, header := range cs.headerCache {
		if header.Height+CleanCacheThreshold < currBlockHeight {
			delete(cs.headerCache, hash)
		}
	}

	for hash, block := range cs.blockCache {
		if block.Header.Height+CleanCacheThreshold < currBlockHeight {
			delete(cs.blockCache, hash)
		}
	}

}

func (cs *ChainStore) InitLedgerStoreWithGenesisBlock(genesisBlock *Block) (uint32, error) {
	hash := genesisBlock.Hash()
	cs.headerIndex[0] = hash

	prefix := []byte{byte(CFG_Version)}
	version, err := cs.st.Get(prefix)
	if err != nil {
		version = []byte{0x00}
	}

	if version[0] == 0x01 {
		// GenesisBlock should exist in chain
		if !cs.IsBlockInStore(hash) {
			estr := fmt.Sprintf("Hash %s is NOT in BlockStore.", hash.ToHexString())
			log.Error(estr)
			return 0, errors.New(estr)
		}
		// Get Current Block
		currentBlockPrefix := []byte{byte(SYS_CurrentBlock)}
		data, err := cs.st.Get(currentBlockPrefix)
		if err != nil {
			return 0, err
		}

		r := bytes.NewReader(data)
		var blockHash Uint256
		blockHash.Deserialize(r)
		cs.currentBlockHeight, err = serialization.ReadUint32(r)
		current_Header_Height := cs.currentBlockHeight

		var listHash Uint256
		iter := cs.st.NewIterator([]byte{byte(IX_HeaderHashList)})
		for iter.Next() {
			rk := bytes.NewReader(iter.Key())
			_, _ = serialization.ReadBytes(rk, 1)
			startNum, err := serialization.ReadUint32(rk)
			if err != nil {
				return 0, err
			}
			r = bytes.NewReader(iter.Value())
			listNum, err := serialization.ReadVarUint(r, 0)
			if err != nil {
				return 0, err
			}

			for i := 0; i < int(listNum); i++ {
				listHash.Deserialize(r)
				cs.headerIndex[startNum+uint32(i)] = listHash
				cs.storedHeaderCount++
			}
		}

		if cs.storedHeaderCount == 0 {
			iter = cs.st.NewIterator([]byte{byte(DATA_BlockHash)})
			for iter.Next() {
				rk := bytes.NewReader(iter.Key())
				_, _ = serialization.ReadBytes(rk, 1)
				listheight, err := serialization.ReadUint32(rk)
				if err != nil {
					return 0, err
				}
				r := bytes.NewReader(iter.Value())
				listHash.Deserialize(r)
				cs.headerIndex[listheight] = listHash
			}
		} else if current_Header_Height >= cs.storedHeaderCount {
			hash = blockHash
			for {
				if hash == cs.headerIndex[cs.storedHeaderCount-1] {
					break
				}

				header, err := cs.GetHeader(hash)
				if err != nil {
					return 0, err
				}

				cs.headerIndex[header.Height] = hash
				hash = header.PrevBlockHash
			}
		}

		return cs.currentBlockHeight, nil

	} else {
		cs.st.NewBatch()
		iter := cs.st.NewIterator(nil)
		for iter.Next() {
			cs.st.BatchDelete(iter.Key())
		}
		iter.Release()

		err := cs.st.BatchCommit()
		if err != nil {
			return 0, err
		}

		// persist genesis block
		cs.persist(genesisBlock)

		// put version to db
		err = cs.st.Put(prefix, []byte{0x01})
		if err != nil {
			return 0, err
		}

		return 0, nil
	}
}

func (cs *ChainStore) IsTxHashDuplicate(txhash Uint256) bool {
	prefix := []byte{byte(DATA_Transaction)}
	_, err_get := cs.st.Get(append(prefix, txhash.ToArray()...))
	return err_get == nil
}

func (cs *ChainStore) IsDoubleSpend(tx *tx.Transaction) bool {
	unspentPrefix := []byte{byte(IX_Unspent)}
NEXT:
	for _, input := range tx.Inputs {
		txhash := input.ReferTxID
		unspentValue, err_get := cs.st.Get(append(unspentPrefix, txhash.ToArray()...))
		if err_get != nil {
			log.Error(err_get)
			return true
		}

		unspents, err := GetUint16Array(unspentValue)
		if err != nil {
			log.Error(err)
			return true
		}
		for _, v := range unspents {
			if v == input.ReferTxOutputIndex {
				continue NEXT // found in unspents
			}
		}
		log.Warningf("Transaction %s refer a spent Output [%x:%d]", tx.Hash(), txhash, input.ReferTxOutputIndex)
		return true // ReferTxOutputIndex not in unspents
	}

	return false
}

func (cs *ChainStore) GetBlockHash(height uint32) (Uint256, error) {
	queryKey := bytes.NewBuffer(nil)
	queryKey.WriteByte(byte(DATA_BlockHash))
	err := serialization.WriteUint32(queryKey, height)

	if err != nil {
		return Uint256{}, err
	}
	blockHash, err_get := cs.st.Get(queryKey.Bytes())
	if err_get != nil {
		//TODO: implement error process
		return Uint256{}, err_get
	}
	blockHash256, err_parse := Uint256ParseFromBytes(blockHash)
	if err_parse != nil {
		return Uint256{}, err_parse
	}

	return blockHash256, nil
}

func (cs *ChainStore) GetBlockByHeight(height uint32) (*Block, error) {
	hash, err := cs.GetBlockHash(height)
	if err != nil {
		return nil, err
	}
	block, err := cs.GetBlock(hash)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (cs *ChainStore) GetCurrentBlockHash() Uint256 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.headerIndex[cs.currentBlockHeight]
}

func (cs *ChainStore) GetContract(codeHash Uint160) ([]byte, error) {
	prefix := []byte{byte(ST_Contract)}
	bData, err := cs.st.Get(append(prefix, codeHash.ToArray()...))
	if err != nil {
		return nil, err
	}

	return bData, nil
}

func (cs *ChainStore) getHeaderWithCache(hash Uint256) (*Header, error) {
	cs.mu.RLock()
	if _, ok := cs.headerCache[hash]; ok {
		cs.mu.RUnlock()
		return cs.headerCache[hash], nil
	}
	cs.mu.RUnlock()

	return cs.GetHeader(hash)
}

func (cs *ChainStore) verifyHeader(header *Header) bool {
	prevHeader, err := cs.getHeaderWithCache(header.PrevBlockHash)
	if err != nil || prevHeader == nil {
		log.Error("[verifyHeader] failed, not found prevHeader.")
		return false
	}

	if prevHeader.Height+1 != header.Height {
		log.Error("[verifyHeader] failed, prevHeader.Height + 1 != header.Height")
		return false
	}

	if prevHeader.Timestamp >= header.Timestamp {
		log.Error("[verifyHeader] failed, prevHeader.Timestamp >= header.Timestamp")
		return false
	}

	//flag, err := validation.VerifySignableData(header)
	//if flag == false || err != nil {
	//	log.Error("[verifyHeader] failed, VerifySignableData failed.")
	//	log.Error(err)
	//	return false
	//}

	return true
}

func (cs *ChainStore) AddHeaders(headers []*Header) error {

	sort.Slice(headers, func(i, j int) bool {
		return headers[i].Height < headers[j].Height
	})

	for i := 0; i < len(headers); i++ {
		cs.taskCh <- &persistHeaderTask{header: headers[i]}
	}

	return nil

}

func (cs *ChainStore) GetHeader(hash Uint256) (*Header, error) {
	cs.mu.RLock()
	if header, ok := cs.headerCache[hash]; ok {
		cs.mu.RUnlock()
		return header, nil
	}
	cs.mu.RUnlock()

	var h = new(Header)
	h.Program = new(program.Program)

	prefix := []byte{byte(DATA_Header)}
	data, err := cs.st.Get(append(prefix, hash.ToArray()...))
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(data)

	// first 8 bytes is sys_fee
	_, err = serialization.ReadUint64(r)
	if err != nil {
		return nil, err
	}

	// Deserialize block data
	err = h.Deserialize(r)
	if err != nil {
		return nil, err
	}

	return h, err
}

func (cs *ChainStore) GetHeaderByHeight(height uint32) (*Header, error) {
	hash := cs.GetHeaderHashByHeight(height)
	return cs.GetHeader(hash)
}

func (cs *ChainStore) SaveAsset(assetId Uint256, asset *Asset) error {
	w := bytes.NewBuffer(nil)

	asset.Serialize(w)

	// generate key
	assetKey := bytes.NewBuffer(nil)
	// add asset prefix.
	assetKey.WriteByte(byte(ST_Info))
	// contact asset id
	assetId.Serialize(assetKey)

	// PUT VALUE
	err := cs.st.BatchPut(assetKey.Bytes(), w.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) GetAsset(hash Uint256) (*Asset, error) {
	asset := new(Asset)
	prefix := []byte{byte(ST_Info)}
	data, err := cs.st.Get(append(prefix, hash.ToArray()...))
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(data)
	asset.Deserialize(r)

	return asset, nil
}

func (cs *ChainStore) SaveName(registrant []byte, name string) error {
	// generate registrant key
	registrantKey := bytes.NewBuffer(nil)
	registrantKey.WriteByte(byte(NS_Registrant))
	serialization.WriteVarBytes(registrantKey, registrant)

	// generate name key
	nameKey := bytes.NewBuffer(nil)
	nameKey.WriteByte(byte(NS_Name))
	serialization.WriteVarString(nameKey, name)

	// PUT VALUE
	w := bytes.NewBuffer(nil)
	serialization.WriteVarString(w, name)
	err := cs.st.BatchPut(registrantKey.Bytes(), w.Bytes())
	if err != nil {
		return err
	}

	w = bytes.NewBuffer(nil)
	serialization.WriteVarBytes(w, registrant)
	err = cs.st.BatchPut(nameKey.Bytes(), w.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) DeleteName(registrant []byte) error {
	name, err := cs.GetName(registrant)
	if err != nil {
		return err
	}

	// generate registrant key
	registrantKey := bytes.NewBuffer(nil)
	registrantKey.WriteByte(byte(NS_Registrant))
	serialization.WriteVarBytes(registrantKey, registrant)

	// generate name key
	nameKey := bytes.NewBuffer(nil)
	nameKey.WriteByte(byte(NS_Name))
	serialization.WriteVarString(nameKey, *name)

	// DELETE VALUE
	err = cs.st.BatchDelete(registrantKey.Bytes())
	if err != nil {
		return err
	}
	err = cs.st.BatchDelete(nameKey.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) GetName(registrant []byte) (*string, error) {
	// generate key
	registrantKey := bytes.NewBuffer(nil)
	registrantKey.WriteByte(byte(NS_Registrant))
	serialization.WriteVarBytes(registrantKey, registrant)

	data, err := cs.st.Get(registrantKey.Bytes())
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(data)

	name, err := serialization.ReadVarString(r)
	if err != nil {
		return nil, err
	}

	return &name, nil
}

func (cs *ChainStore) GetRegistrant(name string) ([]byte, error) {
	// generate key
	nameKey := bytes.NewBuffer(nil)
	nameKey.WriteByte(byte(NS_Name))
	serialization.WriteVarString(nameKey, name)

	data, err := cs.st.Get(nameKey.Bytes())
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(data)

	registrant, err := serialization.ReadVarBytes(r)
	if err != nil {
		return nil, err
	}

	return registrant, nil
}

func generateTopicKey(topic string) []byte {
	topicKey := bytes.NewBuffer(nil)
	topicKey.WriteByte(byte(PS_Topic))
	serialization.WriteVarString(topicKey, topic)

	return topicKey.Bytes()
}

func generateTopicBucketKey(topic string, bucket uint32) []byte {
	topicBucketKey := bytes.NewBuffer(nil)
	topicKey := generateTopicKey(topic)
	topicBucketKey.Write(topicKey)
	serialization.WriteUint32(topicBucketKey, bucket)

	return topicBucketKey.Bytes()
}

func generateSubscriberKey(subscriber []byte, identifier string, topic string, bucket uint32) []byte {
	subscriberKey := bytes.NewBuffer(nil)
	topicBucketKey := generateTopicBucketKey(topic, bucket)
	subscriberKey.Write(topicBucketKey)
	serialization.WriteVarBytes(subscriberKey, subscriber)
	serialization.WriteVarString(subscriberKey, identifier)

	return subscriberKey.Bytes()
}

func (cs *ChainStore) Subscribe(subscriber []byte, identifier string, topic string, bucket uint32, duration uint32, height uint32) error {
	if duration == 0 {
		return nil
	}

	subscriberKey := generateSubscriberKey(subscriber, identifier, topic, bucket)

	// PUT VALUE
	err := cs.st.BatchPut(subscriberKey, []byte{})
	if err != nil {
		return err
	}

	err = cs.ExpireKeyAtBlock(height+duration, subscriberKey)
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) Unsubscribe(subscriber []byte, identifier string, topic string, bucket uint32, duration uint32, height uint32) error {
	if duration == 0 {
		return nil
	}

	subscriberKey := generateSubscriberKey(subscriber, identifier, topic, bucket)

	// DELETE VALUE
	err := cs.st.BatchDelete(subscriberKey)
	if err != nil {
		return err
	}

	err = cs.CancelKeyExpirationAtBlock(height+duration, subscriberKey)
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) IsSubscribed(subscriber []byte, identifier string, topic string, bucket uint32) (bool, error) {
	subscriberKey := generateSubscriberKey(subscriber, identifier, topic, bucket)

	return cs.st.Has(subscriberKey)
}

func (cs *ChainStore) GetSubscribers(topic string, bucket uint32) []string {
	subscribers := make([]string, 0)

	prefix := generateTopicBucketKey(topic, bucket)
	iter := cs.st.NewIterator(prefix)
	for iter.Next() {
		rk := bytes.NewReader(iter.Key())

		// read prefix
		_, _ = serialization.ReadBytes(rk, uint64(len(prefix)))

		subscriber, _ := serialization.ReadVarBytes(rk)
		identifier, _ := serialization.ReadVarString(rk)
		subscriberString := address.MakeAddressString(subscriber, identifier)

		subscribers = append(subscribers, subscriberString)
	}

	return subscribers
}

func (cs *ChainStore) GetSubscribersCount(topic string, bucket uint32) int {
	subscribers := 0

	prefix := generateTopicBucketKey(topic, bucket)
	iter := cs.st.NewIterator(prefix)
	for iter.Next() {
		subscribers++
	}

	return subscribers
}

func (cs *ChainStore) GetFirstAvailableTopicBucket(topic string) int {
	for i := uint32(0); i < tx.BucketsLimit; i++ {
		count := cs.GetSubscribersCount(topic, i)
		if count < tx.SubscriptionsLimit {
			return int(i)
		}
	}

	return -1
}

func (cs *ChainStore) GetTopicBucketsCount(topic string) uint32 {
	lastBucket := uint32(0)

	prefix := generateTopicKey(topic)

	iter := cs.st.NewIterator(prefix)
	for iter.Next() {
		rk := bytes.NewReader(iter.Key())

		// read prefix
		_, _ = serialization.ReadBytes(rk, uint64(len(prefix)))

		bucket, _ := serialization.ReadUint32(rk)

		lastBucket = bucket
	}

	return lastBucket
}

func (cs *ChainStore) ExpireKeyAtBlock(height uint32, key []byte) error {
	expireKey := bytes.NewBuffer(nil)
	expireKey.WriteByte(byte(SYS_ExpireKey))
	serialization.WriteUint32(expireKey, height)
	serialization.WriteVarBytes(expireKey, key)

	err := cs.st.BatchPut(expireKey.Bytes(), []byte{})
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) CancelKeyExpirationAtBlock(height uint32, key []byte) error {
	expireKey := bytes.NewBuffer(nil)
	expireKey.WriteByte(byte(SYS_ExpireKey))
	serialization.WriteUint32(expireKey, height)
	serialization.WriteVarBytes(expireKey, key)

	err := cs.st.BatchDelete(expireKey.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) GetExpiredKeys(height uint32) [][]byte {
	keys := make([][]byte, 0)

	prefix := bytes.NewBuffer(nil)
	prefix.WriteByte(byte(SYS_ExpireKey))
	serialization.WriteUint32(prefix, height)

	iter := cs.st.NewIterator(prefix.Bytes())
	for iter.Next() {
		key := make([]byte, len(iter.Key()))
		copy(key, iter.Key())
		keys = append(keys, key)
	}

	return keys
}

func (cs *ChainStore) RemoveExpiredKey(key []byte) error {
	rk := bytes.NewReader(key)

	// read prefix
	_, err := serialization.ReadBytes(rk, 5)
	if err != nil {
		return err
	}

	expiredKey, err := serialization.ReadVarBytes(rk)
	if err != nil {
		return err
	}

	err = cs.st.BatchDelete(key)
	if err != nil {
		return err
	}

	err = cs.st.BatchDelete(expiredKey)
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) GetTransaction(hash Uint256) (*tx.Transaction, error) {
	t, _, err := cs.getTx(hash)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (cs *ChainStore) getTx(hash Uint256) (*tx.Transaction, uint32, error) {
	key := append([]byte{byte(DATA_Transaction)}, hash.ToArray()...)
	value, err := cs.st.Get(key)
	if err != nil {
		return nil, 0, err
	}

	r := bytes.NewReader(value)
	height, err := serialization.ReadUint32(r)
	if err != nil {
		return nil, 0, err
	}

	var txn tx.Transaction
	if err := txn.Deserialize(r); err != nil {
		return nil, height, err
	}

	return &txn, height, nil
}

func (cs *ChainStore) SaveTransaction(tx *tx.Transaction, height uint32) error {
	txhash := bytes.NewBuffer(nil)
	// add transaction header prefix.
	txhash.WriteByte(byte(DATA_Transaction))
	// get transaction hash
	txHashValue := tx.Hash()
	txHashValue.Serialize(txhash)

	// generate value
	w := bytes.NewBuffer(nil)
	serialization.WriteUint32(w, height)
	tx.Serialize(w)

	// put value
	err := cs.st.BatchPut(txhash.Bytes(), w.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) GetBlock(hash Uint256) (*Block, error) {
	cs.mu.RLock()
	if block, ok := cs.blockCache[hash]; ok {
		cs.mu.RUnlock()
		return block, nil
	}
	cs.mu.RUnlock()

	var b = new(Block)
	b.Header = new(Header)
	b.Header.Program = new(program.Program)

	prefix := []byte{byte(DATA_Header)}
	bHash, err := cs.st.Get(append(prefix, hash.ToArray()...))
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(bHash)
	// first 8 bytes is sys_fee
	_, err = serialization.ReadUint64(r)
	if err != nil {
		return nil, err
	}

	// Deserialize block data
	err = b.FromTrimmedData(r)
	if err != nil {
		return nil, err
	}

	// Deserialize transaction
	for i := 0; i < len(b.Transactions); i++ {
		b.Transactions[i], _, err = cs.getTx(b.Transactions[i].Hash())
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}

func (cs *ChainStore) GetBlockHistory(startHeight, blockNum uint32) map[uint32]Uint256 {
	endHeight := startHeight + blockNum
	if cs.GetHeight() < endHeight {
		endHeight = cs.GetHeight()
	}
	history := make(map[uint32]Uint256)
	for height := startHeight; height < endHeight; height++ {
		blockHash, err := cs.GetBlockHash(height)
		if err != nil {
			return nil
		}
		history[height] = blockHash
	}

	return history
}

func (cs *ChainStore) CheckBlockHistory(history map[uint32]Uint256) (uint32, bool) {
	for height, blockHash := range history {
		h, err := cs.GetBlockHash(height)
		if err != nil {
			return 0, false
		}
		if h.CompareTo(blockHash) != 0 {
			return height, false
		}
	}

	return 0, true
}

// TODO get node voting weight form DB
func (cs *ChainStore) GetVotingWeight(hash Uint160) (int, error) {
	return 1, nil
}

func (cs *ChainStore) persist(b *Block) error {
	utxoUnspents := newUTXOs(cs)
	unspents := make(map[Uint256][]uint16)
	quantities := make(map[Uint256]Fixed64)

	// Get Unspents for every tx
	unspentPrefix := []byte{byte(IX_Unspent)}

	// batch write begin
	cs.st.NewBatch()
	// generate key with DATA_Header prefix
	bhhash := bytes.NewBuffer(nil)
	// add block header prefix.
	bhhash.WriteByte(byte(DATA_Header))
	// calc block hash
	blockHash := b.Hash()
	blockHash.Serialize(bhhash)

	// generate value
	w := bytes.NewBuffer(nil)
	var sysfee uint64 = 0xFFFFFFFFFFFFFFFF
	serialization.WriteUint64(w, sysfee)
	b.Trim(w)

	// BATCH PUT VALUE
	cs.st.BatchPut(bhhash.Bytes(), w.Bytes())

	// generate key with DATA_BlockHash prefix
	bhash := bytes.NewBuffer(nil)
	bhash.WriteByte(byte(DATA_BlockHash))
	err := serialization.WriteUint32(bhash, b.Header.Height)
	if err != nil {
		return err
	}

	// generate value
	hashWriter := bytes.NewBuffer(nil)
	hashValue := b.Header.Hash()
	hashValue.Serialize(hashWriter)

	// BATCH PUT VALUE
	cs.st.BatchPut(bhash.Bytes(), hashWriter.Bytes())

	nLen := len(b.Transactions)
	for i := 0; i < nLen; i++ {
		err = cs.SaveTransaction(b.Transactions[i], b.Header.Height)
		if err != nil {
			return err
		}

		switch b.Transactions[i].TxType {
		case tx.RegisterAsset:
			ar := b.Transactions[i].Payload.(*payload.RegisterAsset)
			err = cs.SaveAsset(b.Transactions[i].Hash(), ar.Asset)
			if err != nil {
				return err
			}
		case tx.Prepaid:
			prepaidPld := b.Transactions[i].Payload.(*payload.Prepaid)
			pHash, err := b.Transactions[i].GetProgramHashes()
			if err != nil || len(pHash) == 0 {
				return err
			}
			err = cs.UpdatePrepaidInfo(pHash[0], prepaidPld.Amount, prepaidPld.Rates)
			if err != nil {
				return err
			}
		case tx.Withdraw:
			withdrawPld := b.Transactions[i].Payload.(*payload.Withdraw)
			// TODO for range output list
			err = cs.UpdateWithdrawInfo(withdrawPld.ProgramHash, b.Transactions[i].Outputs[0].Value)
			if err != nil {
				return err
			}
		case tx.Commit:
			//TODO save POR data
		case tx.IssueAsset:
			results := b.Transactions[i].GetMergedAssetIDValueFromOutputs()
			for assetId, value := range results {
				if _, ok := quantities[assetId]; !ok {
					quantities[assetId] += value
				} else {
					quantities[assetId] = value
				}
			}
		case tx.RegisterName:
			registerNamePayload := b.Transactions[i].Payload.(*payload.RegisterName)
			err = cs.SaveName(registerNamePayload.Registrant, registerNamePayload.Name)
			if err != nil {
				return err
			}
		case tx.DeleteName:
			deleteNamePayload := b.Transactions[i].Payload.(*payload.DeleteName)
			err = cs.DeleteName(deleteNamePayload.Registrant)
			if err != nil {
				return err
			}
		case tx.Subscribe:
			subscribePayload := b.Transactions[i].Payload.(*payload.Subscribe)
			err = cs.Subscribe(subscribePayload.Subscriber, subscribePayload.Identifier, subscribePayload.Topic, subscribePayload.Bucket, subscribePayload.Duration, b.Header.Height)
			if err != nil {
				return err
			}
		}
		for index := 0; index < len(b.Transactions[i].Outputs); index++ {
			output := b.Transactions[i].Outputs[index]
			programHash := output.ProgramHash
			assetId := output.AssetID

			unspent := &tx.UTXOUnspent{
				Txid:  b.Transactions[i].Hash(),
				Index: uint32(index),
				Value: output.Value,
			}

			utxoUnspents.addUTXO(programHash, assetId, b.Header.Height, unspent)
		}

		for index := 0; index < len(b.Transactions[i].Inputs); index++ {
			input := b.Transactions[i].Inputs[index]
			transaction, height, err := cs.getTx(input.ReferTxID)
			if err != nil {
				return err
			}
			index := input.ReferTxOutputIndex
			output := transaction.Outputs[index]
			programHash := output.ProgramHash
			assetId := output.AssetID

			unspent := &tx.UTXOUnspent{
				Txid:  transaction.Hash(),
				Index: uint32(index),
				Value: output.Value,
			}

			if err := utxoUnspents.removeUTXO(programHash, assetId, height, unspent); err != nil {
				return err
			}

		}

		// init unspent in tx
		txhash := b.Transactions[i].Hash()
		for index := 0; index < len(b.Transactions[i].Outputs); index++ {
			unspents[txhash] = append(unspents[txhash], uint16(index))
		}

		// delete unspent when spent in input
		for index := 0; index < len(b.Transactions[i].Inputs); index++ {
			txhash := b.Transactions[i].Inputs[index].ReferTxID

			// if get unspent by utxo
			if _, ok := unspents[txhash]; !ok {
				unspentValue, err_get := cs.st.Get(append(unspentPrefix, txhash.ToArray()...))

				if err_get != nil {
					return err_get
				}

				unspents[txhash], err_get = GetUint16Array(unspentValue)
				if err_get != nil {
					return err_get
				}
			}

			// find Transactions[i].Inputs[index].ReferTxOutputIndex and delete it
			unspentLen := len(unspents[txhash])
			for k, outputIndex := range unspents[txhash] {
				if outputIndex == uint16(b.Transactions[i].Inputs[index].ReferTxOutputIndex) {
					unspents[txhash][k] = unspents[txhash][unspentLen-1]
					unspents[txhash] = unspents[txhash][:unspentLen-1]
					break
				}
			}
		}

	}

	expiredKeys := cs.GetExpiredKeys(b.Header.Height)
	for i := 0; i < len(expiredKeys); i++ {
		err := cs.RemoveExpiredKey(expiredKeys[i])
		if err != nil {
			return err
		}
	}

	// batch put the utxoUnspents
	if err := utxoUnspents.persistUTXOs(); err != nil {
		return err
	}

	// batch put the unspents
	for txhash, value := range unspents {
		unspentKey := bytes.NewBuffer(nil)
		unspentKey.WriteByte(byte(IX_Unspent))
		txhash.Serialize(unspentKey)

		if len(value) == 0 {
			cs.st.BatchDelete(unspentKey.Bytes())
		} else {
			unspentArray := ToByteArray(value)
			cs.st.BatchPut(unspentKey.Bytes(), unspentArray)
		}
	}

	// batch put quantities
	for assetId, value := range quantities {
		quantityKey := bytes.NewBuffer(nil)
		quantityKey.WriteByte(byte(ST_QuantityIssued))
		assetId.Serialize(quantityKey)

		qt, err := cs.GetQuantityIssued(assetId)
		if err != nil {
			return err
		}

		qt = qt + value

		quantityArray := bytes.NewBuffer(nil)
		qt.Serialize(quantityArray)

		cs.st.BatchPut(quantityKey.Bytes(), quantityArray.Bytes())
	}

	currentBlockKey := bytes.NewBuffer(nil)
	currentBlockKey.WriteByte(byte(SYS_CurrentBlock))

	currentBlock := bytes.NewBuffer(nil)
	blockHash.Serialize(currentBlock)
	serialization.WriteUint32(currentBlock, b.Header.Height)

	// BATCH PUT VALUE
	cs.st.BatchPut(currentBlockKey.Bytes(), currentBlock.Bytes())

	err = cs.st.BatchCommit()

	if err != nil {
		return err
	}

	return nil
}

// can only be invoked by backend write goroutine
func (cs *ChainStore) addHeader(header *Header) {
	hash := header.Hash()

	cs.mu.Lock()
	cs.headerCache[header.Hash()] = header
	cs.headerIndex[header.Height] = hash
	cs.mu.Unlock()
}

func (cs *ChainStore) handlePersistHeaderTask(header *Header) {

	if header.Height != cs.GetHeaderHeight()+1 {
		return
	}

	if !cs.verifyHeader(header) {
		return
	}

	cs.addHeader(header)
}

func (cs *ChainStore) SaveBlock(b *Block, ledger *Ledger) error {
	cs.mu.RLock()
	headerHeight := uint32(len(cs.headerIndex))
	currBlockHeight := cs.currentBlockHeight
	cs.mu.RUnlock()

	if b.Header.Height <= currBlockHeight {
		return nil
	}

	if b.Header.Height > headerHeight {
		return errors.New(fmt.Sprintf("Info: [SaveBlock] block height - headerIndex.count >= 1, block height:%d, headerIndex.count:%d",
			b.Header.Height, headerHeight))
	}

	if b.Header.Height == headerHeight {
		//err := VerifyBlock(b, ledger, false)
		//if err != nil {
		//	log.Error("VerifyBlock error!")
		//	return err
		//}

		cs.taskCh <- &persistHeaderTask{header: b.Header}
	} else {
		//flag, err := validation.VerifySignableData(b)
		//if flag == false || err != nil {
		//	log.Error("VerifyBlock error!")
		//	return err
		//}
	}

	cs.taskCh <- &persistBlockTask{block: b, ledger: ledger}
	return nil
}

func (cs *ChainStore) handlePersistBlockTask(b *Block, ledger *Ledger) {
	if b.Header.Height <= cs.currentBlockHeight {
		return
	}

	cs.mu.Lock()
	cs.blockCache[b.Hash()] = b
	cs.mu.Unlock()

	if b.Header.Height < cs.GetHeaderHeight()+1 {
		cs.persistBlocks(ledger)

		cs.st.NewBatch()
		storedHeaderCount := cs.storedHeaderCount
		for cs.currentBlockHeight-storedHeaderCount >= HeaderHashListCount {
			hashBuffer := new(bytes.Buffer)
			serialization.WriteVarUint(hashBuffer, uint64(HeaderHashListCount))
			var hashArray []byte
			for i := 0; i < HeaderHashListCount; i++ {
				index := storedHeaderCount + uint32(i)
				cs.mu.RLock()
				thash := cs.headerIndex[index]
				cs.mu.RUnlock()
				thehash := thash.ToArray()
				hashArray = append(hashArray, thehash...)
			}
			hashBuffer.Write(hashArray)

			hhlPrefix := bytes.NewBuffer(nil)
			hhlPrefix.WriteByte(byte(IX_HeaderHashList))
			serialization.WriteUint32(hhlPrefix, storedHeaderCount)

			cs.st.BatchPut(hhlPrefix.Bytes(), hashBuffer.Bytes())
			storedHeaderCount += HeaderHashListCount
		}

		err := cs.st.BatchCommit()
		if err != nil {
			log.Error("failed to persist header hash list:", err)
			return
		}
		cs.mu.Lock()
		cs.storedHeaderCount = storedHeaderCount
		cs.mu.Unlock()

		cs.clearCache()
	}
}

func (cs *ChainStore) persistBlocks(ledger *Ledger) {
	cs.mu.RLock()
	initHeight := cs.currentBlockHeight + 1
	cs.mu.RUnlock()

	stopHeight := cs.GetHeaderHeight() + 1
	for h := initHeight; h <= stopHeight; h++ {
		cs.mu.RLock()
		hash := cs.headerIndex[h]
		block, ok := cs.blockCache[hash]
		cs.mu.RUnlock()
		if !ok {
			break
		}
		err := cs.persist(block)
		if err != nil {
			log.Error("[persistBlocks]: error to persist block:", err.Error())
			return
		}

		// PersistCompleted event
		ledger.Blockchain.BlockHeight = block.Header.Height
		cs.mu.Lock()
		cs.currentBlockHeight = block.Header.Height
		cs.mu.Unlock()

		ledger.Blockchain.BCEvents.Notify(events.EventBlockPersistCompleted, block)
		log.Infof("# current block height: %d, block hash: %x", block.Header.Height, hash.ToArrayReverse())
	}

}

func (cs *ChainStore) BlockInCache(hash Uint256) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	_, ok := cs.blockCache[hash]
	return ok
}

func (cs *ChainStore) GetQuantityIssued(assetId Uint256) (Fixed64, error) {
	prefix := []byte{byte(ST_QuantityIssued)}
	data, err := cs.st.Get(append(prefix, assetId.ToArray()...))
	var quantity Fixed64
	if err != nil {
		quantity = Fixed64(0)
	} else {
		r := bytes.NewReader(data)
		quantity.Deserialize(r)
	}

	return quantity, nil
}

func (cs *ChainStore) GetUnspent(txid Uint256, index uint16) (*tx.TxnOutput, error) {
	if ok, _ := cs.ContainsUnspent(txid, index); ok {
		Tx, err := cs.GetTransaction(txid)
		if err != nil {
			return nil, err
		}

		return Tx.Outputs[index], nil
	}

	return nil, errors.New("[GetUnspent] NOT ContainsUnspent.")
}

func (cs *ChainStore) ContainsUnspent(txid Uint256, index uint16) (bool, error) {
	unspentPrefix := []byte{byte(IX_Unspent)}
	unspentValue, err := cs.st.Get(append(unspentPrefix, txid.ToArray()...))
	if err != nil {
		return false, err
	}
	unspentArray, err_get := GetUint16Array(unspentValue)
	if err_get != nil {
		return false, err_get
	}
	for i := 0; i < len(unspentArray); i++ {
		if unspentArray[i] == index {
			return true, nil
		}
	}

	return false, nil
}

func (cs *ChainStore) GetCurrentHeaderHash() Uint256 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.headerIndex[uint32(len(cs.headerIndex)-1)]
}

func (cs *ChainStore) GetHeaderHashByHeight(height uint32) Uint256 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.headerIndex[height]
}

func (cs *ChainStore) GetHeaderHeight() uint32 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return uint32(len(cs.headerIndex) - 1)
}

func (cs *ChainStore) GetHeight() uint32 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.currentBlockHeight
}

func (cs *ChainStore) GetHeightByBlockHash(hash Uint256) (uint32, error) {
	header, err := cs.getHeaderWithCache(hash)
	if err == nil {
		return header.Height, nil
	}

	block, err := cs.GetBlock(hash)
	if err != nil {
		return 0, err
	}

	return block.Header.Height, nil
}

func (cs *ChainStore) IsBlockInStore(hash Uint256) bool {
	var b = new(Block)
	b.Header = new(Header)
	b.Header.Program = new(program.Program)

	prefix := []byte{byte(DATA_Header)}
	blockData, err := cs.st.Get(append(prefix, hash.ToArray()...))
	if err != nil {
		return false
	}

	r := bytes.NewReader(blockData)

	// first 8 bytes is sys_fee
	_, err = serialization.ReadUint64(r)
	if err != nil {
		return false
	}

	// Deserialize block data
	err = b.FromTrimmedData(r)
	if err != nil {
		return false
	}

	if b.Header.Height > cs.currentBlockHeight {
		return false
	}

	return true
}

func (cs *ChainStore) GetUnspentFromProgramHash(programHash Uint160, assetid Uint256) ([]*tx.UTXOUnspent, error) {
	prefix := []byte{byte(IX_Unspent_UTXO)}
	key := append(prefix, programHash.ToArray()...)
	key = append(key, assetid.ToArray()...)
	iter := cs.st.NewIterator(key)
	unspents := make([]*tx.UTXOUnspent, 0)
	for iter.Next() {
		r := bytes.NewReader(iter.Value())
		listNum, err := serialization.ReadVarUint(r, 0)
		if err != nil {
			return nil, err
		}

		for i := 0; i < int(listNum); i++ {
			uu := new(tx.UTXOUnspent)
			err := uu.Deserialize(r)
			if err != nil {
				return nil, err
			}

			unspents = append(unspents, uu)
		}

	}

	return unspents, nil
}

func (cs *ChainStore) saveUnspentWithProgramHash(programHash Uint160, assetid Uint256, height uint32, unspents []*tx.UTXOUnspent) error {
	prefix := []byte{byte(IX_Unspent_UTXO)}

	key := append(prefix, programHash.ToArray()...)
	key = append(key, assetid.ToArray()...)
	keyBuffer := bytes.NewBuffer(key)
	if err := serialization.WriteUint32(keyBuffer, height); err != nil {
		return err
	}

	if len(unspents) == 0 {
		cs.st.BatchDelete(keyBuffer.Bytes())
		return nil
	}

	listnum := len(unspents)
	w := bytes.NewBuffer(nil)
	serialization.WriteVarUint(w, uint64(listnum))
	for i := 0; i < listnum; i++ {
		unspents[i].Serialize(w)
	}

	err := cs.st.BatchPut(keyBuffer.Bytes(), w.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) GetUnspentsFromProgramHash(programHash Uint160) (map[Uint256][]*tx.UTXOUnspent, error) {
	uxtoUnspents := make(map[Uint256][]*tx.UTXOUnspent)

	prefix := []byte{byte(IX_Unspent_UTXO)}
	key := append(prefix, programHash.ToArray()...)
	iter := cs.st.NewIterator(key)
	for iter.Next() {
		rk := bytes.NewReader(iter.Key())

		// read prefix
		_, _ = serialization.ReadBytes(rk, 1)
		var ph Uint160
		ph.Deserialize(rk)
		var assetid Uint256
		assetid.Deserialize(rk)

		r := bytes.NewReader(iter.Value())
		listNum, err := serialization.ReadVarUint(r, 0)
		if err != nil {
			return nil, err
		}

		// read unspent list in store
		unspents := make([]*tx.UTXOUnspent, listNum)
		for i := 0; i < int(listNum); i++ {
			uu := new(tx.UTXOUnspent)
			err := uu.Deserialize(r)
			if err != nil {
				return nil, err
			}

			unspents[i] = uu
		}
		uxtoUnspents[assetid] = append(uxtoUnspents[assetid], unspents[:]...)
	}

	return uxtoUnspents, nil
}

func (cs *ChainStore) GetUnspentByHeight(programHash Uint160, assetid Uint256, height uint32) ([]*tx.UTXOUnspent, error) {
	prefix := []byte{byte(IX_Unspent_UTXO)}
	prefix = append(prefix, programHash.ToArray()...)
	prefix = append(prefix, assetid.ToArray()...)

	key := bytes.NewBuffer(prefix)
	if err := serialization.WriteUint32(key, height); err != nil {
		return nil, err
	}
	unspentsData, err := cs.st.Get(key.Bytes())
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(unspentsData)
	listNum, err := serialization.ReadVarUint(r, 0)
	if err != nil {
		return nil, err
	}

	// read unspent list in store
	unspents := make([]*tx.UTXOUnspent, listNum)
	for i := 0; i < int(listNum); i++ {
		uu := new(tx.UTXOUnspent)
		err := uu.Deserialize(r)
		if err != nil {
			return nil, err
		}

		unspents[i] = uu
	}

	return unspents, nil
}

func (cs *ChainStore) GetAssets() map[Uint256]*Asset {
	assets := make(map[Uint256]*Asset)

	iter := cs.st.NewIterator([]byte{byte(ST_Info)})
	for iter.Next() {
		rk := bytes.NewReader(iter.Key())

		// read prefix
		_, _ = serialization.ReadBytes(rk, 1)
		var assetid Uint256
		assetid.Deserialize(rk)
		log.Debugf("[GetAssets] assetid: %x\n", assetid.ToArray())

		asset := new(Asset)
		r := bytes.NewReader(iter.Value())
		asset.Deserialize(r)

		assets[assetid] = asset
	}

	return assets
}

func (cs *ChainStore) GetStorage(key []byte) ([]byte, error) {
	prefix := []byte{byte(ST_Storage)}
	bData, err_get := cs.st.Get(append(prefix, key...))

	if err_get != nil {
		return nil, err_get
	}
	return bData, nil
}

func (cs *ChainStore) UpdatePrepaidInfo(programHash Uint160, amount, rates Fixed64) error {
	var err error
	newAmount := amount
	// key: prefix + programHash
	key := append([]byte{byte(ST_Prepaid)}, programHash.ToArray()...)

	// value: increase existed prepaid amount
	value := bytes.NewBuffer(nil)
	oldAmount, _, _ := cs.GetPrepaidInfo(programHash)
	if oldAmount != nil {
		newAmount = *oldAmount + amount
	}
	err = newAmount.Serialize(value)
	if err != nil {
		return err
	}
	err = rates.Serialize(value)
	if err != nil {
		return err
	}

	err = cs.st.BatchPut(key, value.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) UpdateWithdrawInfo(programHash Uint160, amount Fixed64) error {
	var err error
	newAmount := amount
	// key: prefix + programHash
	key := append([]byte{byte(ST_Prepaid)}, programHash.ToArray()...)

	// value: increase existed prepaid amount
	value := bytes.NewBuffer(nil)
	oldAmount, rates, _ := cs.GetPrepaidInfo(programHash)
	if oldAmount != nil {
		newAmount = *oldAmount - amount
	}
	err = newAmount.Serialize(value)
	if err != nil {
		return err
	}
	err = rates.Serialize(value)
	if err != nil {
		return err
	}

	err = cs.st.BatchPut(key, value.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) GetPrepaidInfo(programHash Uint160) (*Fixed64, *Fixed64, error) {
	var amount, rates Fixed64
	var err error

	key := append([]byte{byte(ST_Prepaid)}, programHash.ToArray()...)
	value, err := cs.st.Get(key)
	if err != nil {
		return nil, nil, err
	}
	r := bytes.NewReader(value)
	err = amount.Deserialize(r)
	if err != nil {
		return nil, nil, err
	}
	err = rates.Deserialize(r)
	if err != nil {
		return nil, nil, err
	}

	return &amount, &rates, nil
}

type UTXOs struct {
	cs       *ChainStore
	unspents map[Uint160]map[Uint256]map[uint32][]*tx.UTXOUnspent
}

func newUTXOs(store *ChainStore) *UTXOs {
	return &UTXOs{
		cs:       store,
		unspents: make(map[Uint160]map[Uint256]map[uint32][]*tx.UTXOUnspent),
	}
}

func (u *UTXOs) addUTXO(programHash Uint160, assetId Uint256, height uint32, unspent *tx.UTXOUnspent) error {
	if _, ok := u.unspents[programHash]; !ok {
		u.unspents[programHash] = make(map[Uint256]map[uint32][]*tx.UTXOUnspent)
	}

	if _, ok := u.unspents[programHash][assetId]; !ok {
		u.unspents[programHash][assetId] = make(map[uint32][]*tx.UTXOUnspent, 0)
	}

	var err error
	if _, ok := u.unspents[programHash][assetId][height]; !ok {
		u.unspents[programHash][assetId][height], err = u.cs.GetUnspentByHeight(programHash, assetId, height)
		if err != nil {
			u.unspents[programHash][assetId][height] = make([]*tx.UTXOUnspent, 0)
		}
	}

	u.unspents[programHash][assetId][height] = append(u.unspents[programHash][assetId][height], unspent)

	return nil
}

func (u *UTXOs) removeUTXO(programHash Uint160, assetId Uint256, height uint32, unspent *tx.UTXOUnspent) error {
	if _, ok := u.unspents[programHash]; !ok {
		u.unspents[programHash] = make(map[Uint256]map[uint32][]*tx.UTXOUnspent)
	}

	if _, ok := u.unspents[programHash][assetId]; !ok {
		u.unspents[programHash][assetId] = make(map[uint32][]*tx.UTXOUnspent)
	}

	var err error
	if _, ok := u.unspents[programHash][assetId][height]; !ok {
		u.unspents[programHash][assetId][height], err = u.cs.GetUnspentByHeight(programHash, assetId, height)
		if err != nil {
			return errors.New(fmt.Sprintf("[programHash:%v, assetId:%v height: %v] has no unspent UTXO.",
				programHash, assetId, height))
		}
	}
	listnum := len(u.unspents[programHash][assetId][height])
	for i := 0; i < listnum; i++ {
		if u.unspents[programHash][assetId][height][i].Txid.CompareTo(unspent.Txid) == 0 &&
			u.unspents[programHash][assetId][height][i].Index == uint32(unspent.Index) {
			u.unspents[programHash][assetId][height][i] = u.unspents[programHash][assetId][height][listnum-1]
			u.unspents[programHash][assetId][height] = u.unspents[programHash][assetId][height][:listnum-1]
			return nil
		}
	}

	return errors.New(fmt.Sprintf("[persist] utxoUnspents NOT find UTXO by txid: %x, index: %d.",
		unspent.Txid, unspent.Index))

}

func (u *UTXOs) persistUTXOs() error {
	for programHash, programHash_value := range u.unspents {
		for assetId, unspents := range programHash_value {
			for height, unspent := range unspents {
				err := u.cs.saveUnspentWithProgramHash(programHash, assetId, height, unspent)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
