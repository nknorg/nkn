package store

import (
	"bytes"
	"container/list"
	"errors"
	"sort"
	"sync"

	. "nkn-core/common"
	"nkn-core/common/log"
	"nkn-core/common/serialization"
	. "nkn-core/core/asset"
	"nkn-core/core/contract/program"
	. "nkn-core/core/ledger"
)

const (
	TaskChanCap    = 4
	MinMemoryNodes = 200
)

type persistTask interface{}

type persistHeaderTask struct {
	header *BlockHeader
}

type persistBlockTask struct {
	block *Block
}

type rollbackBlockTask struct {
	blockHash Uint256
}

type ChainStore struct {
	mu           sync.RWMutex
	headerCache  map[Uint256]*BlockHeader
	headerIdx    *list.List
	ledger       *Ledger
	blockHeight  uint32
	headerHeight uint32
	taskCh       chan persistTask
	quit         chan chan bool
	IStore
}

func NewLedgerStore() (ILedgerStore, error) {
	st, err := NewLevelDBStore("Chain")
	if err != nil {
		return nil, err
	}
	chain := &ChainStore{
		IStore:      st,
		headerCache: map[Uint256]*BlockHeader{},
		headerIdx:   list.New(),
		ledger:      DefaultLedger,
		blockHeight: 0,
		taskCh:      make(chan persistTask, TaskChanCap),
		quit:        make(chan chan bool, 1),
	}

	go chain.loop()

	return chain, nil
}

func (p *ChainStore) SaveBlock(b *Block) error {
	p.taskCh <- &persistBlockTask{block: b}
	return nil
}

func (p *ChainStore) RollbackBlock(blockHash Uint256) error {
	p.taskCh <- &rollbackBlockTask{blockHash: blockHash}

	return nil
}

func (p *ChainStore) AddHeaders(headers []BlockHeader, ledger *Ledger) error {
	sort.Slice(headers, func(i, j int) bool {
		return headers[i].Height < headers[j].Height
	})

	for i := 0; i < len(headers); i++ {
		p.taskCh <- &persistHeaderTask{header: &headers[i]}
	}

	return nil
}

func (p *ChainStore) loop() {
	for {
		select {
		case t := <-p.taskCh:
			switch task := t.(type) {
			case *persistHeaderTask:
				p.handlePersistHeaderTask(task.header)
			case *persistBlockTask:
				p.handlePersistBlockTask(task.block)
			case *rollbackBlockTask:
				p.handleRollbackBlockTask(task.blockHash)
			}
		case closed := <-p.quit:
			closed <- true
			return
		}
	}
}

func (p *ChainStore) handlePersistHeaderTask(header *BlockHeader) {
	if !p.verifyBlockHeader(header) {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	p.headerCache[header.Hash()] = header
	p.headerIdx.PushBack(*header)
	p.headerHeight = header.Height
}

func (p *ChainStore) handlePersistBlockTask(b *Block) {
	p.processBlock(b)
}

func (p *ChainStore) handleRollbackBlockTask(blockHash Uint256) {
	block, err := p.GetBlock(blockHash)
	if err != nil {
		return
	}
	p.rollbackBlock(block)
}

func (p *ChainStore) InitLedgerStoreWithGenesisBlock(genesisBlock *Block) (uint32, error) {
	prefix := []byte{byte(CFG_Version)}
	version, err := p.Get(prefix)
	if err != nil {
		version = []byte{0x00}
	}

	if version[0] == 0x00 {
		p.NewBatch()
		iter := p.NewIterator(nil)
		for iter.Next() {
			p.BatchDelete(iter.Key())
		}
		iter.Release()
		err := p.BatchCommit()
		if err != nil {
			return 0, err
		}

		// processBlock genesis block
		p.processBlock(genesisBlock)

		// write version to db
		err = p.Put(prefix, []byte{0x01})
		if err != nil {
			return 0, err
		}
		return 0, nil
	} else {
		data, err := p.Get([]byte{byte(SYS_CurrentBlock)})
		if err != nil {
			return 0, err
		}
		r := bytes.NewReader(data)
		var blockHash Uint256
		blockHash.Deserialize(r)
		// load current height to memory
		p.blockHeight, err = serialization.ReadUint32(r)
		// TODO load others to memory
		return p.blockHeight, nil
	}
}

func (p *ChainStore) GetBlock(hash Uint256) (*Block, error) {
	var b = new(Block)
	b.Header = new(BlockHeader)
	b.Header.Program = new(program.Program)

	v, err := p.Get(append([]byte{byte(DATA_Header)}, hash.ToArray()...))
	if err != nil {
		return nil, err
	}

	// first 8 bytes is sys_fee
	r := bytes.NewReader(v)
	_, err = serialization.ReadUint64(r)
	if err != nil {
		return nil, err
	}

	err = b.FromTrimmedData(r)
	if err != nil {
		return nil, err
	}

	for _, t := range b.Transactions {
		txn, err := p.GetTransaction(t.Hash())
		if err != nil {
			return nil, err
		}
		*t = *txn
	}

	return b, nil
}

func (p *ChainStore) GetHeader(hash Uint256) (*BlockHeader, error) {
	var header = new(BlockHeader)
	header.Program = new(program.Program)

	v, err := p.Get(append([]byte{byte(DATA_Header)}, hash.ToArray()...))
	if err != nil {
		return nil, err
	}

	// first 8 bytes is sys_fee
	r := bytes.NewReader(v)
	_, err = serialization.ReadUint64(r)
	if err != nil {
		return nil, err
	}

	err = header.Deserialize(r)
	if err != nil {
		return nil, err
	}

	return header, err
}

func (p *ChainStore) GetBlockHeight() uint32 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.blockHeight
}

func (p *ChainStore) GetHeaderHeight() uint32 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.headerHeight
}

func (p *ChainStore) GetBlockHashByHeight(height uint32) (Uint256, error) {
	k := bytes.NewBuffer(nil)
	k.WriteByte(byte(DATA_BlockHash))
	err := serialization.WriteUint32(k, height)
	if err != nil {
		return Uint256{}, err
	}
	v, err := p.Get(k.Bytes())
	if err != nil {
		return Uint256{}, err
	}
	blockHash, err := Uint256ParseFromBytes(v)
	if err != nil {
		return Uint256{}, err
	}

	return blockHash, nil
}

func (p *ChainStore) GetHeaderHashByHeight(height uint32) Uint256 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for e := p.headerIdx.Front(); e != nil; e = e.Next() {
		n := e.Value.(BlockHeader)
		if height == n.Height {
			return n.Hash()
		}
	}

	hash, err := p.GetBlockHashByHeight(height)
	if err != nil {
		return Uint256{}
	}

	return hash
}

func (p *ChainStore) GetCurrentBlockHash() Uint256 {
	hash, err := p.GetBlockHashByHeight(p.blockHeight)
	if err != nil {
		return Uint256{}
	}

	return hash
}

func (p *ChainStore) GetCurrentHeaderHash() Uint256 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	e := p.headerIdx.Back()
	if e != nil {
		n := e.Value.(BlockHeader)
		return n.Hash()
	}

	return p.GetCurrentBlockHash()
}

func (p *ChainStore) GetAsset(hash Uint256) (*Asset, error) {
	asset := new(Asset)
	v, err := p.Get(append([]byte{byte(ST_Info)}, hash.ToArray()...))
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(v)
	asset.Deserialize(r)

	return asset, nil
}

func (p *ChainStore) GetUnspent(txid Uint256, index uint16) (*TxOutput, error) {
	if ok, _ := p.ContainsUnspent(txid, index); ok {
		Tx, err := p.GetTransaction(txid)
		if err != nil {
			return nil, err
		}

		return Tx.Outputs[index], nil
	}

	return nil, errors.New("[GetUnspent] NOT ContainsUnspent.")
}

func (p *ChainStore) GetUnspentFromProgramHash(programHash Uint160, assetid Uint256) ([]*UTXOUnspent, error) {
	k := append([]byte{byte(IX_Unspent_UTXO)}, programHash.ToArray()...)
	k = append(k, assetid.ToArray()...)
	v, err := p.Get(k)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(v)
	listNum, err := serialization.ReadVarUint(r, 0)
	if err != nil {
		return nil, err
	}

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

func (p *ChainStore) GetUnspentsFromProgramHash(programHash Uint160) (map[Uint256][]*UTXOUnspent, error) {
	uxtoUnspents := make(map[Uint256][]*UTXOUnspent)
	k := append([]byte{byte(IX_Unspent_UTXO)}, programHash.ToArray()...)
	iter := p.NewIterator(k)
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

func (p *ChainStore) GetTransaction(hash Uint256) (*Transaction, error) {
	v, err := p.Get(append([]byte{byte(DATA_Transaction)}, hash.ToArray()...))
	if err != nil {
		return nil, err
	}

	// get height
	r := bytes.NewReader(v)
	_, err = serialization.ReadUint32(r)
	if err != nil {
		return nil, err
	}

	txn := new(Transaction)
	err = txn.Deserialize(r)
	if err != nil {
		return nil, err
	}

	return txn, nil
}

func (p *ChainStore) ContainsUnspent(txid Uint256, index uint16) (bool, error) {
	unspentValue, err := p.Get(append([]byte{byte(IX_Unspent)}, txid.ToArray()...))
	if err != nil {
		return false, err
	}

	unspentArray, err := GetUint16Array(unspentValue)
	if err != nil {
		return false, err
	}
	for i := 0; i < len(unspentArray); i++ {
		if unspentArray[i] == index {
			return true, nil
		}
	}

	return false, nil
}

func (p *ChainStore) GetHeaderHashFront() (Uint256, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	e := p.headerIdx.Front()
	if e != nil {
		header := e.Value.(BlockHeader)
		return header.Hash(), nil
	}

	return Uint256{}, errors.New("no element in headerIdx.")
}

func (p *ChainStore) GetHeaderHashNext(prevHash Uint256) (Uint256, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for e := p.headerIdx.Front(); e != nil; e = e.Next() {
		n := e.Value.(BlockHeader)
		h := n.Hash()
		if h.CompareTo(prevHash) == 0 {
			e2 := e.Next()
			if e2 != nil {
				header := e2.Value.(BlockHeader)
				return header.Hash(), nil
			}
		}
	}

	return Uint256{}, errors.New("no element in headerIdx.")
}

func (p *ChainStore) RemoveHeaderListElement(hash Uint256) {
	for e := p.headerIdx.Front(); e != nil; e = e.Next() {
		n := e.Value.(BlockHeader)
		h := n.Hash()
		if h.CompareTo(hash) == 0 {
			p.headerIdx.Remove(e)
		}
	}
}

func (p *ChainStore) IsTxHashDuplicate(txhash Uint256) bool {
	_, err := p.GetTransaction(txhash)
	if err != nil {
		return false
	}

	return true
}

func (p *ChainStore) IsDoubleSpend(tx *Transaction) bool {
	if len(tx.UTXOInputs) == 0 {
		return false
	}
	for i := 0; i < len(tx.UTXOInputs); i++ {
		txhash := tx.UTXOInputs[i].ReferTxID
		unspentValue, err := p.Get(append([]byte{byte(IX_Unspent)}, txhash.ToArray()...))
		if err != nil {
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

func (p *ChainStore) Close() {
	closed := make(chan bool)
	p.quit <- closed
	<-closed

	p.Close()
}

func (p *ChainStore) getHeaderWithCache(hash Uint256) (*BlockHeader, error) {
	for e := p.headerIdx.Front(); e != nil; e = e.Next() {
		n := e.Value.(BlockHeader)
		eh := n.Hash()
		if eh.CompareTo(hash) == 0 {
			return &n, nil
		}
	}

	return p.GetHeader(hash)
}

func (p *ChainStore) removeHeaderFromCache(b *BlockHeader) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for e := p.headerIdx.Front(); e != nil; e = e.Next() {
		n := e.Value.(BlockHeader)
		h := n.Hash()
		if h.CompareTo(b.Hash()) == 0 {
			p.headerIdx.Remove(e)
		}
	}
}

func (p *ChainStore) verifyBlockHeader(header *BlockHeader) bool {
	prevHeader, err := p.getHeaderWithCache(header.PrevBlockHash)
	if err != nil {
		log.Error("[verifyHeader] failed, not found prevHeader.")
		return false
	}

	if prevHeader.Height+1 != header.Height {
		log.Error("[verifyHeader] failed, prevHeader.Height + 1 != header.Height")
		return false
	}

	if prevHeader.Timestamp > header.Timestamp {
		log.Error("[verifyHeader] failed, prevHeader.Timestamp >= header.Timestamp")
		return false
	}

	return true
}

func (p *ChainStore) processBlock(b *Block) error {
	p.NewBatch()
	p.saveTrimmedBlock(b)
	p.saveBlockHash(b)
	p.saveTransactions(b)
	p.saveUnspend(b)
	p.saveCurrentBlock(b)
	p.BatchCommit()

	p.mu.Lock()
	p.blockHeight = b.Header.Height
	p.mu.Unlock()
	p.removeHeaderFromCache(b.Header)

	return nil
}

func (p *ChainStore) rollbackBlock(b *Block) error {
	p.NewBatch()
	p.rollbackTrimemedBlock(b)
	p.rollbackBlockHash(b)
	p.rollbackTransactions(b)
	p.rollbackUnspend(b)
	p.rollbackCurrentBlock(b)
	p.BatchCommit()

	p.mu.Lock()
	p.blockHeight = b.Header.Height - 1
	p.mu.Unlock()

	return nil
}

func (p *ChainStore) saveUnspentWithProgramHash(programHash Uint160, assetid Uint256, unspents []*UTXOUnspent) error {
	k := append([]byte{byte(IX_Unspent_UTXO)}, programHash.ToArray()...)
	k = append(k, assetid.ToArray()...)
	listnum := len(unspents)
	w := bytes.NewBuffer(nil)
	serialization.WriteVarUint(w, uint64(listnum))
	for i := 0; i < listnum; i++ {
		unspents[i].Serialize(w)
	}

	err := p.BatchPut(k, w.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (p *ChainStore) saveTrimmedBlock(b *Block) error {
	k := bytes.NewBuffer(nil)
	k.WriteByte(byte(DATA_Header))
	blockHash := b.Hash()
	blockHash.Serialize(k)

	v := bytes.NewBuffer(nil)
	var sysfee uint64 = 0x0000000000000000
	serialization.WriteUint64(v, sysfee)
	b.Trim(v)

	if err := p.BatchPut(k.Bytes(), v.Bytes()); err != nil {
		return err
	}

	return nil
}

func (p *ChainStore) rollbackTrimemedBlock(b *Block) error {
	k := bytes.NewBuffer(nil)
	k.WriteByte(byte(DATA_Header))
	blockHash := b.Hash()
	blockHash.Serialize(k)

	if err := p.BatchDelete(k.Bytes()); err != nil {
		return err
	}

	return nil
}

func (p *ChainStore) saveBlockHash(b *Block) error {
	k := bytes.NewBuffer(nil)
	k.WriteByte(byte(DATA_BlockHash))
	if err := serialization.WriteUint32(k, b.Header.Height); err != nil {
		return err
	}

	v := bytes.NewBuffer(nil)
	hash := b.Header.Hash()
	hash.Serialize(v)

	if err := p.BatchPut(k.Bytes(), v.Bytes()); err != nil {
		return err
	}

	return nil
}

func (p *ChainStore) rollbackBlockHash(b *Block) error {
	k := bytes.NewBuffer(nil)
	k.WriteByte(byte(DATA_BlockHash))
	if err := serialization.WriteUint32(k, b.Header.Height); err != nil {
		return err
	}

	if err := p.BatchDelete(k.Bytes()); err != nil {
		return err
	}

	return nil
}

func (p *ChainStore) saveCurrentBlock(b *Block) error {
	k := bytes.NewBuffer(nil)
	k.WriteByte(byte(SYS_CurrentBlock))

	v := bytes.NewBuffer(nil)
	blockHash := b.Hash()
	blockHash.Serialize(v)
	serialization.WriteUint32(v, b.Header.Height)

	if err := p.BatchPut(k.Bytes(), v.Bytes()); err != nil {
		return err
	}

	return nil
}

func (p *ChainStore) rollbackCurrentBlock(b *Block) error {
	k := bytes.NewBuffer(nil)
	k.WriteByte(byte(SYS_CurrentBlock))

	v := bytes.NewBuffer(nil)
	blockHash := b.Header.PrevBlockHash
	blockHash.Serialize(v)
	serialization.WriteUint32(v, b.Header.Height-1)

	if err := p.BatchPut(k.Bytes(), v.Bytes()); err != nil {
		return err
	}

	return nil
}

func (p *ChainStore) saveTransactions(b *Block) error {
	for _, txn := range b.Transactions {
		k := bytes.NewBuffer(nil)
		k.WriteByte(byte(DATA_Transaction))
		txnHash := txn.Hash()
		txnHash.Serialize(k)

		v := bytes.NewBuffer(nil)
		serialization.WriteUint32(v, b.Header.Height)
		txn.Serialize(v)

		err := p.BatchPut(k.Bytes(), v.Bytes())
		if err != nil {
			return err
		}
		//if txn.TxType == ledger.RegisterAsset {
		//regPayload := txn.Payload.(*payload.RegisterAsset)
		//if err := p.saveAsset(txn.Hash(), regPayload.Asset); err != nil {
		//	return err
		//}
		//}
	}
	return nil
}

func (p *ChainStore) rollbackTransactions(b *Block) error {
	for _, txn := range b.Transactions {
		k := bytes.NewBuffer(nil)
		k.WriteByte(byte(DATA_Transaction))
		txnHash := txn.Hash()
		txnHash.Serialize(k)

		if err := p.BatchDelete(k.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

func (p *ChainStore) saveAsset(assetId Uint256, asset *Asset) error {
	k := bytes.NewBuffer(nil)
	k.WriteByte(byte(ST_Info))
	assetId.Serialize(k)

	v := bytes.NewBuffer(nil)
	asset.Serialize(v)

	err := p.BatchPut(k.Bytes(), v.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (p *ChainStore) rollbackAsset(assetId Uint256) error {
	k := bytes.NewBuffer(nil)
	k.WriteByte(byte(ST_Info))
	assetId.Serialize(k)

	if err := p.BatchDelete(k.Bytes()); err != nil {
		return err
	}

	return nil
}

func (p *ChainStore) saveUnspend(b *Block) error {
	unspents := make(map[Uint256][]uint16)
	for _, txn := range b.Transactions {
		txnHash := txn.Hash()
		for index := range txn.Outputs {
			unspents[txnHash] = append(unspents[txnHash], uint16(index))
		}
		for index, input := range txn.UTXOInputs {
			referTxnHash := input.ReferTxID
			if _, ok := unspents[referTxnHash]; !ok {
				unspentValue, err := p.Get(append([]byte{byte(IX_Unspent)}, referTxnHash.ToArray()...))
				if err != nil {
					return err
				}
				unspents[referTxnHash], err = GetUint16Array(unspentValue)
				if err != nil {
					return err
				}
			}

			unspentLen := len(unspents[referTxnHash])
			for k, outputIndex := range unspents[referTxnHash] {
				if outputIndex == uint16(txn.UTXOInputs[index].ReferTxOutputIndex) {
					unspents[referTxnHash][k] = unspents[referTxnHash][unspentLen-1]
					unspents[referTxnHash] = unspents[referTxnHash][:unspentLen-1]
					break
				}
			}
		}
	}

	for txnhash, value := range unspents {
		k := bytes.NewBuffer(nil)
		k.WriteByte(byte(IX_Unspent))
		txnhash.Serialize(k)

		if len(value) == 0 {
			p.BatchDelete(k.Bytes())
		} else {
			unspentArray := ToByteArray(value)
			p.BatchPut(k.Bytes(), unspentArray)
		}
	}

	return nil
}

func (p *ChainStore) rollbackUnspend(b *Block) error {
	prefix := []byte{byte(IX_Unspent)}
	unspents := make(map[Uint256][]uint16)
	for _, txn := range b.Transactions {
		txnHash := txn.Hash()
		if err := p.BatchDelete(append(prefix, txnHash.ToArray()...)); err != nil {
			return err
		}
		for _, input := range txn.UTXOInputs {
			referTxnHash := input.ReferTxID
			referTxnOutIndex := input.ReferTxOutputIndex
			if _, ok := unspents[referTxnHash]; !ok {
				var err error
				unspentValue, _ := p.Get(append(prefix, referTxnHash.ToArray()...))
				if len(unspentValue) != 0 {
					unspents[referTxnHash], err = GetUint16Array(unspentValue)
					if err != nil {
						return err
					}
				}
			}
			unspents[referTxnHash] = append(unspents[referTxnHash], referTxnOutIndex)
		}
	}

	for txhash, value := range unspents {
		k := bytes.NewBuffer(nil)
		k.WriteByte(byte(IX_Unspent))
		txhash.Serialize(k)

		if len(value) == 0 {
			p.BatchDelete(k.Bytes())
		} else {
			unspentArray := ToByteArray(value)
			p.BatchPut(k.Bytes(), unspentArray)
		}
	}

	return nil
}
