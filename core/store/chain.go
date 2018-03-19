package store

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	. "nkn-core/common"
	"nkn-core/common/log"
	"nkn-core/common/serialization"
	. "nkn-core/core/asset"
	"nkn-core/core/contract/program"
	. "nkn-core/core/ledger"
	"nkn-core/events"
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
	header *BlockHeader
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
	headerCache map[Uint256]*BlockHeader

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
		headerCache:        map[Uint256]*BlockHeader{},
		currentBlockHeight: 0,
		storedHeaderCount:  0,
		taskCh:             make(chan persistTask, TaskChanCap),
		quit:               make(chan chan bool, 1),
	}

	go chain.loop()

	return chain, nil
}

func (self *ChainStore) Close() {
	closed := make(chan bool)
	self.quit <- closed
	<-closed

	self.st.Close()
}

func (self *ChainStore) loop() {
	for {
		select {
		case t := <-self.taskCh:
			now := time.Now()
			switch task := t.(type) {
			case *persistHeaderTask:
				self.handlePersistHeaderTask(task.header)
				tcall := float64(time.Now().Sub(now)) / float64(time.Second)
				log.Debugf("handle header exetime: %g \n", tcall)

			case *persistBlockTask:
				self.handlePersistBlockTask(task.block, task.ledger)
				tcall := float64(time.Now().Sub(now)) / float64(time.Second)
				log.Debugf("handle block exetime: %g num transactions:%d \n", tcall, len(task.block.Transactions))
			}

		case closed := <-self.quit:
			closed <- true
			return
		}
	}
}

// can only be invoked by backend write goroutine
func (self *ChainStore) clearCache() {
	self.mu.Lock()
	defer self.mu.Unlock()

	currBlockHeight := self.currentBlockHeight
	for hash, header := range self.headerCache {
		if header.Height+CleanCacheThreshold < currBlockHeight {
			delete(self.headerCache, hash)
		}
	}

	for hash, block := range self.blockCache {
		if block.Header.Height+CleanCacheThreshold < currBlockHeight {
			delete(self.blockCache, hash)
		}
	}

}

func (bd *ChainStore) InitLedgerStoreWithGenesisBlock(genesisBlock *Block) (uint32, error) {

	hash := genesisBlock.Hash()
	bd.headerIndex[0] = hash
	log.Debugf("listhash genesis: %x\n", hash)

	prefix := []byte{byte(CFG_Version)}
	version, err := bd.st.Get(prefix)
	if err != nil {
		version = []byte{0x00}
	}

	if version[0] == 0x01 {
		// GenesisBlock should exist in chain
		// Or the bookkeepers are not consistent with the chain
		if !bd.IsBlockInStore(hash) {
			return 0, errors.New("bookkeepers are not consistent with the chain")
		}
		// Get Current Block
		currentBlockPrefix := []byte{byte(SYS_CurrentBlock)}
		data, err := bd.st.Get(currentBlockPrefix)
		if err != nil {
			return 0, err
		}

		r := bytes.NewReader(data)
		var blockHash Uint256
		blockHash.Deserialize(r)
		bd.currentBlockHeight, err = serialization.ReadUint32(r)
		current_Header_Height := bd.currentBlockHeight

		log.Debugf("blockHash: %x\n", blockHash.ToArray())

		var listHash Uint256
		iter := bd.st.NewIterator([]byte{byte(IX_HeaderHashList)})
		for iter.Next() {
			rk := bytes.NewReader(iter.Key())
			// read prefix
			_, _ = serialization.ReadBytes(rk, 1)
			startNum, err := serialization.ReadUint32(rk)
			if err != nil {
				return 0, err
			}
			log.Debugf("start index: %d\n", startNum)

			r = bytes.NewReader(iter.Value())
			listNum, err := serialization.ReadVarUint(r, 0)
			if err != nil {
				return 0, err
			}

			for i := 0; i < int(listNum); i++ {
				listHash.Deserialize(r)
				bd.headerIndex[startNum+uint32(i)] = listHash
				bd.storedHeaderCount++
				//log.Debug( fmt.Sprintf( "listHash %d: %x\n", startNum+uint32(i), listHash ) )
			}
		}

		if bd.storedHeaderCount == 0 {
			iter = bd.st.NewIterator([]byte{byte(DATA_BlockHash)})
			for iter.Next() {
				rk := bytes.NewReader(iter.Key())
				// read prefix
				_, _ = serialization.ReadBytes(rk, 1)
				listheight, err := serialization.ReadUint32(rk)
				if err != nil {
					return 0, err
				}
				//log.Debug(fmt.Sprintf( "DATA_BlockHash block height: %d\n", listheight ))

				r := bytes.NewReader(iter.Value())
				listHash.Deserialize(r)
				//log.Debug(fmt.Sprintf( "DATA_BlockHash block hash: %x\n", listHash ))

				bd.headerIndex[listheight] = listHash
			}
		} else if current_Header_Height >= bd.storedHeaderCount {
			hash = blockHash
			for {
				if hash == bd.headerIndex[bd.storedHeaderCount-1] {
					break
				}

				header, err := bd.GetHeader(hash)
				if err != nil {
					return 0, err
				}

				//log.Debug(fmt.Sprintf( "header height: %d\n", header.BlockHeader.Height ))
				//log.Debug(fmt.Sprintf( "header hash: %x\n", hash ))

				bd.headerIndex[header.Height] = hash
				hash = header.PrevBlockHash
			}
		}

		return bd.currentBlockHeight, nil

	} else {
		// batch delete old data
		bd.st.NewBatch()
		iter := bd.st.NewIterator(nil)
		for iter.Next() {
			bd.st.BatchDelete(iter.Key())
		}
		iter.Release()

		err := bd.st.BatchCommit()
		if err != nil {
			return 0, err
		}

		// persist genesis block
		bd.persist(genesisBlock)

		// put version to db
		err = bd.st.Put(prefix, []byte{0x01})
		if err != nil {
			return 0, err
		}

		return 0, nil
	}
}

func (bd *ChainStore) InitLedgerStore(l *Ledger) error {
	// TODO: InitLedgerStore
	return nil
}

func (bd *ChainStore) IsTxHashDuplicate(txhash Uint256) bool {
	prefix := []byte{byte(DATA_Transaction)}
	_, err_get := bd.st.Get(append(prefix, txhash.ToArray()...))
	if err_get != nil {
		return false
	} else {
		return true
	}
}

func (bd *ChainStore) IsDoubleSpend(tx *Transaction) bool {
	if len(tx.UTXOInputs) == 0 {
		return false
	}

	unspentPrefix := []byte{byte(IX_Unspent)}
	for i := 0; i < len(tx.UTXOInputs); i++ {
		txhash := tx.UTXOInputs[i].ReferTxID
		unspentValue, err_get := bd.st.Get(append(unspentPrefix, txhash.ToArray()...))
		if err_get != nil {
			return true
		}

		unspents, _ := GetUint16Array(unspentValue)
		findFlag := false
		for k := 0; k < len(unspents); k++ {
			if unspents[k] == tx.UTXOInputs[i].ReferTxOutputIndex {
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

func (bd *ChainStore) GetBlockHash(height uint32) (Uint256, error) {
	queryKey := bytes.NewBuffer(nil)
	queryKey.WriteByte(byte(DATA_BlockHash))
	err := serialization.WriteUint32(queryKey, height)

	if err != nil {
		return Uint256{}, err
	}
	blockHash, err_get := bd.st.Get(queryKey.Bytes())
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

func (bd *ChainStore) GetCurrentBlockHash() Uint256 {
	bd.mu.RLock()
	defer bd.mu.RUnlock()

	return bd.headerIndex[bd.currentBlockHeight]
}

func (bd *ChainStore) GetContract(codeHash Uint160) ([]byte, error) {
	prefix := []byte{byte(ST_Contract)}
	bData, err_get := bd.st.Get(append(prefix, codeHash.ToArray()...))
	if err_get != nil {
		//TODO: implement error process
		return nil, err_get
	}

	log.Debug("GetContract Data: ", bData)

	return bData, nil
}

func (bd *ChainStore) getHeaderWithCache(hash Uint256) *BlockHeader {
	if _, ok := bd.headerCache[hash]; ok {
		return bd.headerCache[hash]
	}

	header, _ := bd.GetHeader(hash)

	return header
}

func (bd *ChainStore) verifyHeader(header *BlockHeader) bool {
	prevHeader := bd.getHeaderWithCache(header.PrevBlockHash)

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

	flag, err := VerifySignableData(header)
	if flag == false || err != nil {
		log.Error("[verifyHeader] failed, VerifySignableData failed.")
		log.Error(err)
		return false
	}

	return true
}

func (self *ChainStore) AddHeaders(headers []BlockHeader, ledger *Ledger) error {

	sort.Slice(headers, func(i, j int) bool {
		return headers[i].Height < headers[j].Height
	})

	for i := 0; i < len(headers); i++ {
		self.taskCh <- &persistHeaderTask{header: &headers[i]}
	}

	return nil

}

func (bd *ChainStore) GetHeader(hash Uint256) (*BlockHeader, error) {
	bd.mu.RLock()
	if header, ok := bd.headerCache[hash]; ok {
		bd.mu.RUnlock()
		return header, nil
	}
	bd.mu.RUnlock()

	var h *BlockHeader = new(BlockHeader)
	h.Program = new(program.Program)

	prefix := []byte{byte(DATA_Header)}
	log.Debug("GetHeader Data:", hash.ToArray())
	data, err_get := bd.st.Get(append(prefix, hash.ToArray()...))
	//log.Debug( "Get Header Data: %x\n",  data )
	if err_get != nil {
		//TODO: implement error process
		return nil, err_get
	}

	r := bytes.NewReader(data)

	// first 8 bytes is sys_fee
	sysfee, err := serialization.ReadUint64(r)
	if err != nil {
		return nil, err
	}
	log.Debug(fmt.Sprintf("sysfee: %d\n", sysfee))

	// Deserialize block data
	err = h.Deserialize(r)
	if err != nil {
		return nil, err
	}

	return h, err
}

func (bd *ChainStore) SaveAsset(assetId Uint256, asset *Asset) error {
	w := bytes.NewBuffer(nil)

	asset.Serialize(w)

	// generate key
	assetKey := bytes.NewBuffer(nil)
	// add asset prefix.
	assetKey.WriteByte(byte(ST_Info))
	// contact asset id
	assetId.Serialize(assetKey)

	log.Debug(fmt.Sprintf("asset key: %x\n", assetKey))

	// PUT VALUE
	err := bd.st.BatchPut(assetKey.Bytes(), w.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (bd *ChainStore) GetAsset(hash Uint256) (*Asset, error) {
	log.Debug(fmt.Sprintf("GetAsset Hash: %x\n", hash))

	asset := new(Asset)

	prefix := []byte{byte(ST_Info)}
	data, err_get := bd.st.Get(append(prefix, hash.ToArray()...))

	log.Debug(fmt.Sprintf("GetAsset Data: %x\n", data))
	if err_get != nil {
		//TODO: implement error process
		return nil, err_get
	}

	r := bytes.NewReader(data)
	asset.Deserialize(r)

	return asset, nil
}

func (bd *ChainStore) GetTransaction(hash Uint256) (*Transaction, error) {
	log.Debugf("GetTransaction Hash: %x\n", hash)

	t := new(Transaction)
	err := bd.getTx(t, hash)

	if err != nil {
		return nil, err
	}

	return t, nil
}

func (bd *ChainStore) getTx(tx *Transaction, hash Uint256) error {
	prefix := []byte{byte(DATA_Transaction)}
	tHash, err_get := bd.st.Get(append(prefix, hash.ToArray()...))
	if err_get != nil {
		//TODO: implement error process
		return err_get
	}

	r := bytes.NewReader(tHash)

	// get height
	_, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}

	// Deserialize Transaction
	err = tx.Deserialize(r)

	return err
}

func (bd *ChainStore) SaveTransaction(tx *Transaction, height uint32) error {
	//////////////////////////////////////////////////////////////
	// generate key with DATA_Transaction prefix
	txhash := bytes.NewBuffer(nil)
	// add transaction header prefix.
	txhash.WriteByte(byte(DATA_Transaction))
	// get transaction hash
	txHashValue := tx.Hash()
	txHashValue.Serialize(txhash)
	log.Debug(fmt.Sprintf("transaction header + hash: %x\n", txhash))

	// generate value
	w := bytes.NewBuffer(nil)
	serialization.WriteUint32(w, height)
	tx.Serialize(w)
	log.Debug(fmt.Sprintf("transaction tx data: %x\n", w))

	// put value
	err := bd.st.BatchPut(txhash.Bytes(), w.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (bd *ChainStore) GetBlock(hash Uint256) (*Block, error) {
	bd.mu.RLock()
	if block, ok := bd.blockCache[hash]; ok {
		bd.mu.RUnlock()
		return block, nil
	}
	bd.mu.RUnlock()

	var b *Block = new(Block)

	b.Header = new(BlockHeader)
	b.Header.Program = new(program.Program)

	prefix := []byte{byte(DATA_Header)}
	bHash, err_get := bd.st.Get(append(prefix, hash.ToArray()...))
	if err_get != nil {
		//TODO: implement error process
		return nil, err_get
	}

	r := bytes.NewReader(bHash)

	// first 8 bytes is sys_fee
	_, err := serialization.ReadUint64(r)
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
		err = bd.getTx(b.Transactions[i], b.Transactions[i].Hash())
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}

func (bd *ChainStore) persist(b *Block) error {
	utxoUnspents := make(map[Uint160]map[Uint256][]*UTXOUnspent)
	unspents := make(map[Uint256][]uint16)

	///////////////////////////////////////////////////////////////
	// Get Unspents for every tx
	unspentPrefix := []byte{byte(IX_Unspent)}

	///////////////////////////////////////////////////////////////
	// batch write begin
	bd.st.NewBatch()

	//////////////////////////////////////////////////////////////
	// generate key with DATA_Header prefix
	bhhash := bytes.NewBuffer(nil)
	// add block header prefix.
	bhhash.WriteByte(byte(DATA_Header))
	// calc block hash
	blockHash := b.Hash()
	blockHash.Serialize(bhhash)
	log.Debugf("block header + hash: %x\n", bhhash)

	// generate value
	w := bytes.NewBuffer(nil)
	var sysfee uint64 = 0xFFFFFFFFFFFFFFFF
	serialization.WriteUint64(w, sysfee)
	b.Trim(w)

	// BATCH PUT VALUE
	bd.st.BatchPut(bhhash.Bytes(), w.Bytes())

	//////////////////////////////////////////////////////////////
	// generate key with DATA_BlockHash prefix
	bhash := bytes.NewBuffer(nil)
	bhash.WriteByte(byte(DATA_BlockHash))
	err := serialization.WriteUint32(bhash, b.Header.Height)
	if err != nil {
		return err
	}
	log.Debugf("DATA_BlockHash table key: %x\n", bhash)

	// generate value
	hashWriter := bytes.NewBuffer(nil)
	hashValue := b.Hash()
	hashValue.Serialize(hashWriter)
	log.Debugf("DATA_BlockHash table value: %x\n", hashValue)

	// BATCH PUT VALUE
	bd.st.BatchPut(bhash.Bytes(), hashWriter.Bytes())

	//////////////////////////////////////////////////////////////
	// save transactions to leveldb
	nLen := len(b.Transactions)

	for i := 0; i < nLen; i++ {

		err = bd.SaveTransaction(b.Transactions[i], b.Header.Height)
		if err != nil {
			return err
		}

		for index := 0; index < len(b.Transactions[i].Outputs); index++ {
			output := b.Transactions[i].Outputs[index]
			programHash := output.ProgramHash
			assetId := output.AssetID

			// add utxoUnspent
			if _, ok := utxoUnspents[programHash]; !ok {
				utxoUnspents[programHash] = make(map[Uint256][]*UTXOUnspent)
			}

			if _, ok := utxoUnspents[programHash][assetId]; !ok {
				utxoUnspents[programHash][assetId], err = bd.GetUnspentFromProgramHash(programHash, assetId)
				if err != nil {
					utxoUnspents[programHash][assetId] = make([]*UTXOUnspent, 0)
				}
			}

			unspent := new(UTXOUnspent)
			unspent.Txid = b.Transactions[i].Hash()
			unspent.Index = uint32(index)
			unspent.Value = output.Value

			utxoUnspents[programHash][assetId] = append(utxoUnspents[programHash][assetId], unspent)
		}

		for index := 0; index < len(b.Transactions[i].UTXOInputs); index++ {
			input := b.Transactions[i].UTXOInputs[index]
			transaction, err := bd.GetTransaction(input.ReferTxID)
			if err != nil {
				return err
			}
			index := input.ReferTxOutputIndex
			output := transaction.Outputs[index]
			programHash := output.ProgramHash
			assetId := output.AssetID

			// delete utxoUnspent
			if _, ok := utxoUnspents[programHash]; !ok {
				utxoUnspents[programHash] = make(map[Uint256][]*UTXOUnspent)
			}

			if _, ok := utxoUnspents[programHash][assetId]; !ok {
				utxoUnspents[programHash][assetId], err = bd.GetUnspentFromProgramHash(programHash, assetId)
				if err != nil {
					return errors.New(fmt.Sprintf("[persist] utxoUnspents programHash:%v, assetId:%v has no unspent UTXO.", programHash, assetId))
				}
			}

			flag := false
			listnum := len(utxoUnspents[programHash][assetId])
			for i := 0; i < listnum; i++ {
				if utxoUnspents[programHash][assetId][i].Txid.CompareTo(transaction.Hash()) == 0 && utxoUnspents[programHash][assetId][i].Index == uint32(index) {
					utxoUnspents[programHash][assetId][i] = utxoUnspents[programHash][assetId][listnum-1]
					utxoUnspents[programHash][assetId] = utxoUnspents[programHash][assetId][:listnum-1]

					flag = true
					break
				}
			}

			if !flag {
				return errors.New(fmt.Sprintf("[persist] utxoUnspents NOT find UTXO by txid: %x, index: %d.", transaction.Hash(), index))
			}

		}

		// init unspent in tx
		txhash := b.Transactions[i].Hash()
		for index := 0; index < len(b.Transactions[i].Outputs); index++ {
			unspents[txhash] = append(unspents[txhash], uint16(index))
		}

		// delete unspent when spent in input
		for index := 0; index < len(b.Transactions[i].UTXOInputs); index++ {
			txhash := b.Transactions[i].UTXOInputs[index].ReferTxID

			// if get unspent by utxo
			if _, ok := unspents[txhash]; !ok {
				unspentValue, err_get := bd.st.Get(append(unspentPrefix, txhash.ToArray()...))

				if err_get != nil {
					return err_get
				}

				unspents[txhash], err_get = GetUint16Array(unspentValue)
				if err_get != nil {
					return err_get
				}
			}

			// find Transactions[i].UTXOInputs[index].ReferTxOutputIndex and delete it
			unspentLen := len(unspents[txhash])
			for k, outputIndex := range unspents[txhash] {
				if outputIndex == uint16(b.Transactions[i].UTXOInputs[index].ReferTxOutputIndex) {
					unspents[txhash][k] = unspents[txhash][unspentLen-1]
					unspents[txhash] = unspents[txhash][:unspentLen-1]
					break
				}
			}
		}
	}
	///////////////////////////////////////////////////////
	//*/

	// batch put the utxoUnspents
	for programHash, programHash_value := range utxoUnspents {
		for assetId, unspents := range programHash_value {
			err := bd.saveUnspentWithProgramHash(programHash, assetId, unspents)
			if err != nil {
				return err
			}
		}
	}

	// batch put the unspents
	for txhash, value := range unspents {
		unspentKey := bytes.NewBuffer(nil)
		unspentKey.WriteByte(byte(IX_Unspent))
		txhash.Serialize(unspentKey)

		if len(value) == 0 {
			bd.st.BatchDelete(unspentKey.Bytes())
		} else {
			unspentArray := ToByteArray(value)
			bd.st.BatchPut(unspentKey.Bytes(), unspentArray)
		}
	}

	currentBlockKey := bytes.NewBuffer(nil)
	currentBlockKey.WriteByte(byte(SYS_CurrentBlock))

	currentBlock := bytes.NewBuffer(nil)
	blockHash.Serialize(currentBlock)
	serialization.WriteUint32(currentBlock, b.Header.Height)

	// BATCH PUT VALUE
	bd.st.BatchPut(currentBlockKey.Bytes(), currentBlock.Bytes())

	if err != nil {
		return err
	}

	err = bd.st.BatchCommit()

	if err != nil {
		return err
	}

	return nil
}

// can only be invoked by backend write goroutine
func (bd *ChainStore) addHeader(header *BlockHeader) {

	log.Debugf("addHeader(), Height=%d\n", header.Height)

	hash := header.Hash()

	bd.mu.Lock()
	bd.headerCache[header.Hash()] = header
	bd.headerIndex[header.Height] = hash
	bd.mu.Unlock()

	log.Debug("[addHeader]: finish, header height:", header.Height)
}

func (self *ChainStore) handlePersistHeaderTask(header *BlockHeader) {

	if header.Height != uint32(len(self.headerIndex)) {
		return
	}

	if !self.verifyHeader(header) {
		return
	}

	self.addHeader(header)
}

func (self *ChainStore) SaveBlock(b *Block, ledger *Ledger) error {
	log.Debug("SaveBlock()")

	self.mu.RLock()
	headerHeight := uint32(len(self.headerIndex))
	currBlockHeight := self.currentBlockHeight
	self.mu.RUnlock()

	if b.Header.Height <= currBlockHeight {
		return nil
	}

	if b.Header.Height > headerHeight {
		return errors.New(fmt.Sprintf("Info: [SaveBlock] block height - headerIndex.count >= 1, block height:%d, headerIndex.count:%d",
			b.Header.Height, headerHeight))
	}

	if b.Header.Height == headerHeight {
		err := VerifyBlock(b, ledger, false)
		if err != nil {
			log.Error("VerifyBlock error!")
			return err
		}

		self.taskCh <- &persistHeaderTask{header: b.Header}
	} else {
		flag, err := VerifySignableData(b)
		if flag == false || err != nil {
			log.Error("VerifyBlock error!")
			return err
		}
	}

	self.taskCh <- &persistBlockTask{block: b, ledger: ledger}
	return nil
}

func (self *ChainStore) handlePersistBlockTask(b *Block, ledger *Ledger) {
	if b.Header.Height <= self.currentBlockHeight {
		return
	}

	self.mu.Lock()
	self.blockCache[b.Hash()] = b
	self.mu.Unlock()

	if b.Header.Height < uint32(len(self.headerIndex)) {
		self.persistBlocks(ledger)

		self.st.NewBatch()
		storedHeaderCount := self.storedHeaderCount
		for self.currentBlockHeight-storedHeaderCount >= HeaderHashListCount {
			hashBuffer := new(bytes.Buffer)
			serialization.WriteVarUint(hashBuffer, uint64(HeaderHashListCount))
			var hashArray []byte
			for i := 0; i < HeaderHashListCount; i++ {
				index := storedHeaderCount + uint32(i)
				thash := self.headerIndex[index]
				thehash := thash.ToArray()
				hashArray = append(hashArray, thehash...)
			}
			hashBuffer.Write(hashArray)

			hhlPrefix := bytes.NewBuffer(nil)
			hhlPrefix.WriteByte(byte(IX_HeaderHashList))
			serialization.WriteUint32(hhlPrefix, storedHeaderCount)

			self.st.BatchPut(hhlPrefix.Bytes(), hashBuffer.Bytes())
			storedHeaderCount += HeaderHashListCount
		}

		err := self.st.BatchCommit()
		if err != nil {
			log.Error("failed to persist header hash list:", err)
			return
		}
		self.mu.Lock()
		self.storedHeaderCount = storedHeaderCount
		self.mu.Unlock()

		self.clearCache()
	}
}

func (bd *ChainStore) persistBlocks(ledger *Ledger) {
	stopHeight := uint32(len(bd.headerIndex))
	for h := bd.currentBlockHeight + 1; h <= stopHeight; h++ {
		hash := bd.headerIndex[h]
		block, ok := bd.blockCache[hash]
		if !ok {
			break
		}
		err := bd.persist(block)
		if err != nil {
			log.Fatal("[persistBlocks]: error to persist block:", err.Error())
			return
		}

		// PersistCompleted event
		ledger.Blockchain.BlockHeight = block.Header.Height
		bd.mu.Lock()
		bd.currentBlockHeight = block.Header.Height
		bd.mu.Unlock()

		ledger.Blockchain.BCEvents.Notify(events.EventBlockPersistCompleted, block)
		log.Tracef("The latest block height:%d, block hash: %x", block.Header.Height, hash)
	}

}

func (bd *ChainStore) BlockInCache(hash Uint256) bool {
	bd.mu.RLock()
	defer bd.mu.RUnlock()

	_, ok := bd.blockCache[hash]
	return ok
}

func (bd *ChainStore) GetUnspent(txid Uint256, index uint16) (*TxOutput, error) {
	if ok, _ := bd.ContainsUnspent(txid, index); ok {
		Tx, err := bd.GetTransaction(txid)
		if err != nil {
			return nil, err
		}

		return Tx.Outputs[index], nil
	}

	return nil, errors.New("[GetUnspent] NOT ContainsUnspent.")
}

func (bd *ChainStore) ContainsUnspent(txid Uint256, index uint16) (bool, error) {
	unspentPrefix := []byte{byte(IX_Unspent)}
	unspentValue, err_get := bd.st.Get(append(unspentPrefix, txid.ToArray()...))

	if err_get != nil {
		return false, err_get
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

func (bd *ChainStore) GetCurrentHeaderHash() Uint256 {
	bd.mu.RLock()
	defer bd.mu.RUnlock()
	return bd.headerIndex[uint32(len(bd.headerIndex)-1)]
}

func (bd *ChainStore) GetHeaderHashByHeight(height uint32) Uint256 {
	bd.mu.RLock()
	defer bd.mu.RUnlock()

	return bd.headerIndex[height]
}

func (bd *ChainStore) GetHeaderHeight() uint32 {
	bd.mu.RLock()
	defer bd.mu.RUnlock()

	return uint32(len(bd.headerIndex) - 1)
}

func (bd *ChainStore) GetHeight() uint32 {
	bd.mu.RLock()
	defer bd.mu.RUnlock()

	return bd.currentBlockHeight
}

func (bd *ChainStore) IsBlockInStore(hash Uint256) bool {

	var b *Block = new(Block)

	b.Header = new(BlockHeader)
	b.Header.Program = new(program.Program)

	prefix := []byte{byte(DATA_Header)}
	blockData, err_get := bd.st.Get(append(prefix, hash.ToArray()...))
	if err_get != nil {
		return false
	}

	r := bytes.NewReader(blockData)

	// first 8 bytes is sys_fee
	_, err := serialization.ReadUint64(r)
	if err != nil {
		return false
	}

	// Deserialize block data
	err = b.FromTrimmedData(r)
	if err != nil {
		return false
	}

	if b.Header.Height > bd.currentBlockHeight {
		return false
	}

	return true
}

func (bd *ChainStore) GetUnspentFromProgramHash(programHash Uint160, assetid Uint256) ([]*UTXOUnspent, error) {

	prefix := []byte{byte(IX_Unspent_UTXO)}

	key := append(prefix, programHash.ToArray()...)
	key = append(key, assetid.ToArray()...)
	unspentsData, err := bd.st.Get(key)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(unspentsData)
	listNum, err := serialization.ReadVarUint(r, 0)
	if err != nil {
		return nil, err
	}

	//log.Trace(fmt.Printf("[getUnspentFromProgramHash] listNum: %d, unspentsData: %x\n", listNum, unspentsData ))

	// read unspent list in store
	unspents := make([]*UTXOUnspent, listNum)
	for i := 0; i < int(listNum); i++ {
		uu := new(UTXOUnspent)
		err := uu.Deserialize(r)
		if err != nil {
			return nil, err
		}

		unspents[i] = uu
	}

	return unspents, nil
}

func (bd *ChainStore) saveUnspentWithProgramHash(programHash Uint160, assetid Uint256, unspents []*UTXOUnspent) error {
	prefix := []byte{byte(IX_Unspent_UTXO)}

	key := append(prefix, programHash.ToArray()...)
	key = append(key, assetid.ToArray()...)

	listnum := len(unspents)
	w := bytes.NewBuffer(nil)
	serialization.WriteVarUint(w, uint64(listnum))
	for i := 0; i < listnum; i++ {
		unspents[i].Serialize(w)
	}

	// BATCH PUT VALUE
	err := bd.st.BatchPut(key, w.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (bd *ChainStore) GetUnspentsFromProgramHash(programHash Uint160) (map[Uint256][]*UTXOUnspent, error) {
	uxtoUnspents := make(map[Uint256][]*UTXOUnspent)

	prefix := []byte{byte(IX_Unspent_UTXO)}
	key := append(prefix, programHash.ToArray()...)
	iter := bd.st.NewIterator(key)
	for iter.Next() {
		rk := bytes.NewReader(iter.Key())

		// read prefix
		_, _ = serialization.ReadBytes(rk, 1)
		var ph Uint160
		ph.Deserialize(rk)
		var assetid Uint256
		assetid.Deserialize(rk)
		log.Tracef("[GetUnspentsFromProgramHash] assetid: %x\n", assetid.ToArray())

		r := bytes.NewReader(iter.Value())
		listNum, err := serialization.ReadVarUint(r, 0)
		if err != nil {
			return nil, err
		}

		// read unspent list in store
		unspents := make([]*UTXOUnspent, listNum)
		for i := 0; i < int(listNum); i++ {
			uu := new(UTXOUnspent)
			err := uu.Deserialize(r)
			if err != nil {
				return nil, err
			}

			unspents[i] = uu
		}
		uxtoUnspents[assetid] = unspents
	}

	return uxtoUnspents, nil
}

func (bd *ChainStore) GetAssets() map[Uint256]*Asset {
	assets := make(map[Uint256]*Asset)

	iter := bd.st.NewIterator([]byte{byte(ST_Info)})
	for iter.Next() {
		rk := bytes.NewReader(iter.Key())

		// read prefix
		_, _ = serialization.ReadBytes(rk, 1)
		var assetid Uint256
		assetid.Deserialize(rk)
		log.Tracef("[GetAssets] assetid: %x\n", assetid.ToArray())

		asset := new(Asset)
		r := bytes.NewReader(iter.Value())
		asset.Deserialize(r)

		assets[assetid] = asset
	}

	return assets
}

func (bd *ChainStore) GetStorage(key []byte) ([]byte, error) {
	prefix := []byte{byte(ST_Storage)}
	bData, err_get := bd.st.Get(append(prefix, key...))

	if err_get != nil {
		return nil, err_get
	}
	return bData, nil
}
