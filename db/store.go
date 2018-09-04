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
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/events"
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

func (cs *ChainStore) InitLedgerStoreWithGenesisBlock(genesisBlock *Block, defaultBookKeeper []*crypto.PubKey) (uint32, error) {
	hash := genesisBlock.Hash()
	cs.headerIndex[0] = hash

	prefix := []byte{byte(CFG_Version)}
	version, err := cs.st.Get(prefix)
	if err != nil {
		version = []byte{0x00}
	}

	if version[0] == 0x01 {
		// GenesisBlock should exist in chain
		// Or the bookkeepers are not consistent with the chain
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
		sort.Sort(crypto.PubKeySlice(defaultBookKeeper))

		// currBookKeeper key
		bkListKey := bytes.NewBuffer(nil)
		bkListKey.WriteByte(byte(SYS_CurrentBookKeeper))

		// currBookKeeper value
		bkListValue := bytes.NewBuffer(nil)
		serialization.WriteUint8(bkListValue, uint8(len(defaultBookKeeper)))
		for k := 0; k < len(defaultBookKeeper); k++ {
			defaultBookKeeper[k].Serialize(bkListValue)
		}

		// nextBookKeeper value
		serialization.WriteUint8(bkListValue, uint8(len(defaultBookKeeper)))
		for k := 0; k < len(defaultBookKeeper); k++ {
			defaultBookKeeper[k].Serialize(bkListValue)
		}

		// defaultBookKeeper put value
		cs.st.Put(bkListKey.Bytes(), bkListValue.Bytes())

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
	if err_get != nil {
		return false
	} else {
		return true
	}
}

func (cs *ChainStore) IsDoubleSpend(tx *tx.Transaction) bool {
	if len(tx.Inputs) == 0 {
		return false
	}

	unspentPrefix := []byte{byte(IX_Unspent)}
	for i := 0; i < len(tx.Inputs); i++ {
		txhash := tx.Inputs[i].ReferTxID
		unspentValue, err_get := cs.st.Get(append(unspentPrefix, txhash.ToArray()...))
		if err_get != nil {
			return true
		}

		unspents, _ := GetUint16Array(unspentValue)
		findFlag := false
		for k := 0; k < len(unspents); k++ {
			if unspents[k] == tx.Inputs[i].ReferTxOutputIndex {
				findFlag = true
				break
			}
		}

		if !findFlag {
			return true
		}
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

func (cs *ChainStore) GetCurrentBlockHash() Uint256 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.headerIndex[cs.currentBlockHeight]
}

func (cs *ChainStore) getHeaderWithCache(hash Uint256) *Header {
	if _, ok := cs.headerCache[hash]; ok {
		return cs.headerCache[hash]
	}

	header, _ := cs.GetHeader(hash)

	return header
}

func (cs *ChainStore) verifyHeader(header *Header) bool {
	prevHeader := cs.getHeaderWithCache(header.PrevBlockHash)

	if prevHeader == nil {
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

func (cs *ChainStore) AddHeaders(headers []Header, ledger *Ledger) error {

	sort.Slice(headers, func(i, j int) bool {
		return headers[i].Height < headers[j].Height
	})

	for i := 0; i < len(headers); i++ {
		cs.taskCh <- &persistHeaderTask{header: &headers[i]}
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

func (cs *ChainStore) GetBookKeeperList() ([]*crypto.PubKey, []*crypto.PubKey, error) {
	prefix := []byte{byte(SYS_CurrentBookKeeper)}
	bkListValue, err := cs.st.Get(prefix)
	if err != nil {
		return nil, nil, err
	}

	r := bytes.NewReader(bkListValue)

	// first 1 bytes is length of list
	currCount, err := serialization.ReadUint8(r)
	if err != nil {
		return nil, nil, err
	}

	var currBookKeeper = make([]*crypto.PubKey, currCount)
	for i := uint8(0); i < currCount; i++ {
		bk := new(crypto.PubKey)
		err := bk.Deserialize(r)
		if err != nil {
			return nil, nil, err
		}

		currBookKeeper[i] = bk
	}

	nextCount, err := serialization.ReadUint8(r)
	if err != nil {
		return nil, nil, err
	}

	var nextBookKeeper = make([]*crypto.PubKey, nextCount)
	for i := uint8(0); i < nextCount; i++ {
		bk := new(crypto.PubKey)
		err := bk.Deserialize(r)
		if err != nil {
			return nil, nil, err
		}

		nextBookKeeper[i] = bk
	}

	return currBookKeeper, nextBookKeeper, nil
}

func (cs *ChainStore) persist(b *Block) error {
	batch := NewBatchStore(cs)
	errChan := make(chan error, 8)

	go func(b *Block, ch chan error) { ch <- batch.storeBlock(b) }(b, errChan)
	go func(txs []*tx.Transaction, ch chan error) { ch <- batch.storeAssets(txs) }(b.Transactions, errChan)
	go func(txs []*tx.Transaction, ch chan error) { ch <- batch.storeQuantityIssued(txs) }(b.Transactions, errChan)
	go func(txs []*tx.Transaction, ch chan error) { ch <- batch.storePrepaid(txs) }(b.Transactions, errChan)
	go func(txs []*tx.Transaction, ch chan error) { ch <- batch.storeWithdraw(txs) }(b.Transactions, errChan)
	go func(txs []*tx.Transaction, ch chan error) { ch <- batch.storeBookkeepers(txs) }(b.Transactions, errChan)
	go func(txs []*tx.Transaction, ch chan error) { ch <- batch.storeUnspentIndexs(txs) }(b.Transactions, errChan)
	go func(height uint32, txs []*tx.Transaction, ch chan error) {
		ch <- batch.storeUTXOs(height, txs)
	}(b.Header.Height, b.Transactions, errChan)

	var err error
	for i := 0; i < 8; i++ {
		if chanErr := <-errChan; chanErr != nil && err == nil {
			err = chanErr
		}
	}

	close(errChan)
	if err != nil {
		return err
	}

	return batch.Commit()
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

	if header.Height != uint32(len(cs.headerIndex)) {
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

	if b.Header.Height < uint32(len(cs.headerIndex)) {
		cs.persistBlocks(ledger)

		cs.st.NewBatch()
		storedHeaderCount := cs.storedHeaderCount
		for cs.currentBlockHeight-storedHeaderCount >= HeaderHashListCount {
			hashBuffer := new(bytes.Buffer)
			serialization.WriteVarUint(hashBuffer, uint64(HeaderHashListCount))
			var hashArray []byte
			for i := 0; i < HeaderHashListCount; i++ {
				index := storedHeaderCount + uint32(i)
				thash := cs.headerIndex[index]
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
	stopHeight := uint32(len(cs.headerIndex))
	for h := cs.currentBlockHeight + 1; h <= stopHeight; h++ {
		hash := cs.headerIndex[h]
		block, ok := cs.blockCache[hash]
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

func (cs *ChainStore) getUnspentIndex(txid Uint256) ([]uint16, error) {
	unspentValue, err := cs.st.Get(unspentIndexKey(txid))
	if err != nil {
		return []uint16{}, err
	}

	unspentArray, err := GetUint16Array(unspentValue)
	if err != nil {
		return []uint16{}, err
	}

	return unspentArray, nil
}
