package db

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/asset"
	"github.com/nknorg/nkn/core/ledger"
	tx "github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/util/log"
)

type ChainStore struct {
	db                 IStore
	headerCache        *HeaderCache
	blockCache         map[common.Uint256]ledger.Block
	currentBlockHeight uint32
	currentBlockHash   common.Uint256
	mu                 sync.RWMutex
}

func NewLedgerStore() (ledger.ILedgerStore, error) {
	st, err := NewLevelDBStore("Chain")
	if err != nil {
		return nil, err
	}

	chain := &ChainStore{
		db:                 st,
		blockCache:         map[common.Uint256]ledger.Block{},
		headerCache:        NewHeaderCache(),
		currentBlockHeight: 0,
		currentBlockHash:   common.Uint256{},
	}

	return chain, nil
}

func (cs *ChainStore) InitLedgerStoreWithGenesisBlock(genesisBlock *ledger.Block, defaultBookKeeper []*crypto.PubKey) (uint32, error) {
	version, err := cs.db.Get(versionKey())
	if err != nil {
		version = []byte{0x00}
	}

	if version[0] == 0x01 {
		fmt.Println("<----------------exist db----------------------->")
		if !cs.IsBlockInStore(genesisBlock.Hash()) {
			return 0, errors.New("genesisBlock is NOT in BlockStore.")
		}

		if cs.currentBlockHash, cs.currentBlockHeight, err = cs.getCurrentBlockHash(); err != nil {
			return 0, err
		}

		currentHeader, _ := cs.getHeader(cs.currentBlockHash)
		cs.headerCache.AddHeaderToCache(currentHeader)

		return cs.currentBlockHeight, nil
	} else {
		fmt.Println("<----------------no db----------------------->")
		batch := cs.db.NewSepBatch()
		iter := cs.db.NewIterator(nil)
		for iter.Next() {
			batch.BatchDelete(iter.Key())
		}
		iter.Release()
		if err := batch.BatchCommit(); err != nil {
			return 0, err
		}

		sort.Sort(crypto.PubKeySlice(defaultBookKeeper))

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
		cs.db.Put(bookkeeperKey(), bkListValue.Bytes())

		if err := cs.persist(genesisBlock); err != nil {
			return 0, err
		}

		// put version to db
		if err = cs.db.Put(versionKey(), []byte{0x01}); err != nil {
			return 0, err
		}

		cs.headerCache.AddHeaderToCache(genesisBlock.Header)
		cs.currentBlockHash = genesisBlock.Hash()
		cs.currentBlockHeight = 0

		return 0, nil
	}
}

func (cs *ChainStore) Close() {
	cs.db.Close()
}

func (cs *ChainStore) getTransaction(hash common.Uint256) (*tx.Transaction, uint32, error) {
	value, err := cs.db.Get(transactionKey(hash))
	if err != nil {
		return nil, 0, err
	}

	r := bytes.NewReader(value)
	height, err := serialization.ReadUint32(r)
	if err != nil {
		return nil, 0, err
	}

	txn := new(tx.Transaction)
	if err := txn.Deserialize(r); err != nil {
		return nil, height, err
	}

	return txn, height, nil
}

func (cs *ChainStore) getBlock(hash common.Uint256) (*ledger.Block, error) {
	bHash, err := cs.db.Get(headerKey(hash))
	if err != nil {
		return nil, err
	}

	b := new(ledger.Block)
	if err = b.FromTrimmedData(bytes.NewReader(bHash)); err != nil {
		return nil, err
	}

	for i := 0; i < len(b.Transactions); i++ {
		if b.Transactions[i], _, err = cs.getTransaction(b.Transactions[i].Hash()); err != nil {
			return nil, err
		}
	}

	return b, nil
}

func (cs *ChainStore) getBlockHash(height uint32) (common.Uint256, error) {
	if blockHash, err := cs.db.Get(blockhashKey(height)); err != nil {
		return common.Uint256{}, err
	} else {
		return common.Uint256ParseFromBytes(blockHash)
	}
}

func (cs *ChainStore) getHeader(hash common.Uint256) (*ledger.Header, error) {
	if data, err := cs.db.Get(headerKey(hash)); err != nil {
		return nil, err
	} else {
		h := new(ledger.Header)
		err := h.Deserialize(bytes.NewReader(data))
		return h, err
	}
}

func (cs *ChainStore) getCurrentBlockHash() (common.Uint256, uint32, error) {
	if data, err := cs.db.Get(currentBlockHashKey()); err != nil {
		return common.Uint256{}, 0, err
	} else {
		var blockHash common.Uint256
		r := bytes.NewReader(data)
		blockHash.Deserialize(r)
		currentHeight, err := serialization.ReadUint32(r)
		return blockHash, currentHeight, err
	}
}

func (cs *ChainStore) getAsset(hash common.Uint256) (*asset.Asset, error) {
	if data, err := cs.db.Get(assetKey(hash)); err != nil {
		return nil, err
	} else {
		asset := new(asset.Asset)
		err := asset.Deserialize(bytes.NewReader(data))
		return asset, err
	}
}

func (cs *ChainStore) getAssets() map[common.Uint256]*asset.Asset {
	assets := make(map[common.Uint256]*asset.Asset)
	iter := iteratorAsset(cs.db)

	for iter.Next() {
		var assetid common.Uint256
		var asset asset.Asset
		assetid.Deserialize(bytes.NewReader(iter.Key()[1:]))
		asset.Deserialize(bytes.NewReader(iter.Value()))
		assets[assetid] = &asset
	}

	return assets
}

func (cs *ChainStore) getBookKeeperList() ([]*crypto.PubKey, []*crypto.PubKey, error) {
	bkListValue, err := cs.db.Get(bookkeeperKey())
	if err != nil {
		return nil, nil, err
	}

	r := bytes.NewReader(bkListValue)
	currCount, err := serialization.ReadUint8(r)
	if err != nil {
		return nil, nil, err
	}

	var currBookKeeper = make([]*crypto.PubKey, currCount)
	for i := uint8(0); i < currCount; i++ {
		bk := new(crypto.PubKey)
		if err := bk.Deserialize(r); err != nil {
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
		if err := bk.Deserialize(r); err != nil {
			return nil, nil, err
		}
		nextBookKeeper[i] = bk
	}

	return currBookKeeper, nextBookKeeper, nil
}

func (cs *ChainStore) getQuantityIssued(assetId common.Uint256) (common.Fixed64, error) {
	if data, err := cs.db.Get(issuseQuantKey(assetId)); err != nil {
		return common.Fixed64(0), nil
	} else {
		var quantity common.Fixed64
		quantity.Deserialize(bytes.NewReader(data))
		return quantity, nil
	}
}

func (cs *ChainStore) getPrepaidInfo(programHash common.Uint160) (*common.Fixed64, *common.Fixed64, error) {
	if value, err := cs.db.Get(prepaidKey(programHash)); err != nil {
		return nil, nil, err
	} else {
		var amount, rates common.Fixed64
		r := bytes.NewReader(value)
		if err := amount.Deserialize(r); err != nil {
			return nil, nil, err
		}
		if err := rates.Deserialize(r); err != nil {
			return nil, nil, err
		}

		return &amount, &rates, nil
	}
}

func (cs *ChainStore) getUnspentFromProgramHash(programHash common.Uint160, assetid common.Uint256) ([]*tx.UTXOUnspent, error) {
	iter := iteratorUTXO(cs.db, append(programHash.ToArray(), assetid.ToArray()...))
	unspents := make([]*tx.UTXOUnspent, 0)
	for iter.Next() {
		r := bytes.NewReader(iter.Value())
		listNum, err := serialization.ReadVarUint(r, 0)
		if err != nil {
			return nil, err
		}

		for i := 0; i < int(listNum); i++ {
			uu := new(tx.UTXOUnspent)
			if err := uu.Deserialize(r); err != nil {
				return nil, err
			}

			unspents = append(unspents, uu)
		}
	}

	return unspents, nil
}

func (cs *ChainStore) getUnspentsFromProgramHash(programHash common.Uint160) (map[common.Uint256][]*tx.UTXOUnspent, error) {
	uxtoUnspents := make(map[common.Uint256][]*tx.UTXOUnspent)
	iter := iteratorUTXO(cs.db, programHash.ToArray())
	for iter.Next() {
		var assetid common.Uint256
		assetid.Deserialize(bytes.NewReader(iter.Key()[1+common.UINT160SIZE:]))

		r := bytes.NewReader(iter.Value())
		listNum, err := serialization.ReadVarUint(r, 0)
		if err != nil {
			return nil, err
		}

		unspents := make([]*tx.UTXOUnspent, listNum)
		for i := 0; i < int(listNum); i++ {
			uu := new(tx.UTXOUnspent)
			if err := uu.Deserialize(r); err != nil {
				return nil, err
			}

			unspents[i] = uu
		}
		uxtoUnspents[assetid] = append(uxtoUnspents[assetid], unspents[:]...)
	}

	return uxtoUnspents, nil
}

func (cs *ChainStore) isTxHashDuplicate(txhash common.Uint256) bool {
	if _, err := cs.db.Get(transactionKey(txhash)); err != nil {
		return false
	}

	return true
}

func (cs *ChainStore) getUnspentIndex(txid common.Uint256) ([]uint16, error) {
	unspentValue, err := cs.db.Get(unspentIndexKey(txid))
	if err != nil {
		return []uint16{}, err
	}

	unspentArray, err := common.GetUint16Array(unspentValue)
	if err != nil {
		return []uint16{}, err
	}

	return unspentArray, nil
}

func (cs *ChainStore) IsBlockInStore(hash common.Uint256) bool {
	if header, err := cs.getHeader(hash); err != nil || header.Height > cs.currentBlockHeight {
		return false
	}

	return true
}

func (cs *ChainStore) IsTxHashDuplicate(txhash common.Uint256) bool {
	return cs.isTxHashDuplicate(txhash)
}

func (cs *ChainStore) IsDoubleSpend(tx *tx.Transaction) bool {
	if len(tx.Inputs) == 0 {
		return false
	}

	unspentPrefix := []byte{byte(IX_Unspent)}
	for i := 0; i < len(tx.Inputs); i++ {
		txhash := tx.Inputs[i].ReferTxID
		unspentValue, err_get := cs.db.Get(append(unspentPrefix, txhash.ToArray()...))
		if err_get != nil {
			return true
		}

		unspents, _ := common.GetUint16Array(unspentValue)
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

func (cs *ChainStore) CheckBlockHistory(history map[uint32]common.Uint256) (uint32, bool) {
	for height, blockHash := range history {
		if h, err := cs.getBlockHash(height); err != nil && h.CompareTo(blockHash) != 0 {
			return height, false
		}
	}

	return 0, true
}

func (cs *ChainStore) GetCurrentBlockHash() common.Uint256 {
	return cs.currentBlockHash
}

func (cs *ChainStore) GetBlockHash(height uint32) (common.Uint256, error) {
	return cs.getBlockHash(height)
}

func (cs *ChainStore) GetTransaction(hash common.Uint256) (*tx.Transaction, error) {
	if t, _, err := cs.getTransaction(hash); err != nil {
		return nil, err
	} else {
		return t, nil
	}
}

func (cs *ChainStore) GetBlock(hash common.Uint256) (*ledger.Block, error) {
	return cs.getBlock(hash)
}

func (cs *ChainStore) GetHeader(hash common.Uint256) (*ledger.Header, error) {
	return cs.getHeader(hash)
}

func (cs *ChainStore) GetAsset(hash common.Uint256) (*asset.Asset, error) {
	return cs.getAsset(hash)
}

func (cs *ChainStore) GetAssets() map[common.Uint256]*asset.Asset {
	return cs.getAssets()
}

func (cs *ChainStore) GetBlockHistory(startHeight, blockNum uint32) map[uint32]common.Uint256 {
	endHeight := startHeight + blockNum
	if cs.GetHeight() < endHeight {
		endHeight = cs.GetHeight()
	}
	history := make(map[uint32]common.Uint256)
	for height := startHeight; height < endHeight; height++ {
		blockHash, err := cs.getBlockHash(height)
		if err != nil {
			return nil
		}
		history[height] = blockHash
	}

	return history
}

func (cs *ChainStore) GetHeightByBlockHash(hash common.Uint256) (uint32, error) {
	if block, err := cs.getBlock(hash); err != nil {
		return 0, err
	} else {
		return block.Header.Height, nil
	}
}

func (cs *ChainStore) GetBookKeeperList() ([]*crypto.PubKey, []*crypto.PubKey, error) {
	return cs.getBookKeeperList()
}

func (cs *ChainStore) GetQuantityIssued(assetId common.Uint256) (common.Fixed64, error) {
	return cs.getQuantityIssued(assetId)
}

func (cs *ChainStore) GetPrepaidInfo(programHash common.Uint160) (*common.Fixed64, *common.Fixed64, error) {
	return cs.getPrepaidInfo(programHash)
}

func (cs *ChainStore) GetUnspentFromProgramHash(programHash common.Uint160, assetid common.Uint256) ([]*tx.UTXOUnspent, error) {
	return cs.getUnspentFromProgramHash(programHash, assetid)
}

func (cs *ChainStore) GetUnspentsFromProgramHash(programHash common.Uint160) (map[common.Uint256][]*tx.UTXOUnspent, error) {
	return cs.getUnspentsFromProgramHash(programHash)
}

func (cs *ChainStore) SaveBlock(b *ledger.Block) error {
	cs.Dump()
	if b.Header.Height != cs.currentBlockHeight+1 {
		cs.mu.Lock()
		for hash, block := range cs.blockCache {
			if block.Header.Height <= cs.currentBlockHeight {
				delete(cs.blockCache, hash)
			}
		}
		cs.blockCache[b.Hash()] = *b
		cs.mu.Unlock()

		return nil
	}

	if err := cs.persistBlock(b); err != nil {
		return err
	}

	if err := cs.persistCachedBlocks(b); err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) persistBlock(b *ledger.Block) error {
	if _, err := cs.headerCache.GetCachedHeader(b.Header.PrevBlockHash); err != nil {
		return errors.New("not found prevHeader.")
	}

	//if err := VerifyBlock(b, ledger, false); err != nil {
	//	log.Error("VerifyBlock error!")
	//	return err
	//}

	if err := cs.persist(b); err != nil {
		log.Error("error to persist block:", err.Error())
		return err
	}

	return nil
}

func (cs *ChainStore) persistCachedBlocks(prevBlock *ledger.Block) error {
	processBlocks := []*ledger.Block{prevBlock}
	for len(processBlocks) > 0 {
		processBlock := processBlocks[0]
		processBlocks = processBlocks[1:]
		for hash, block := range cs.blockCache {
			if block.Header.PrevBlockHash.CompareTo(processBlock.Hash()) == 0 {
				delete(cs.blockCache, hash)
				if err := cs.persistBlock(&block); err != nil {
					return err
				}
				processBlocks = append(processBlocks, &block)
				break
			}
		}
	}

	return nil
}

func (cs *ChainStore) AddHeaders(headers []ledger.Header) error {
	sort.Slice(headers, func(i, j int) bool {
		return headers[i].Height < headers[j].Height
	})

	for _, header := range headers {
		if header.Height <= cs.headerCache.GetCurrentCachedHeight() {
			continue
		}

		prevHeader, err := cs.headerCache.GetCachedHeader(header.PrevBlockHash)
		if err != nil {
			return errors.New("[verifyHeader] failed, not found prevHeader.")
		}

		if prevHeader.Height+1 != header.Height {
			return errors.New("[verifyHeader] failed, prevHeader.Height + 1 != header.Height")
		}

		if prevHeader.Timestamp >= header.Timestamp {
			return errors.New("[verifyHeader] failed, prevHeader.Timestamp >= header.Timestamp")
		}

		//flag, err := validation.VerifySignableData(header)
		//if flag == false || err != nil {
		//	log.Error("[verifyHeader] failed, VerifySignableData failed.")
		//	log.Error(err)
		//	return false
		//}

		//	cs.headerCache.RemoveCachedHeader(cs.currentBlockHeight - 1)
		cs.headerCache.AddHeaderToCache(&header)
	}

	return nil

}

func (cs *ChainStore) BlockInCache(hash common.Uint256) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	_, ok := cs.blockCache[hash]
	return ok
}

func (cs *ChainStore) GetHeight() uint32 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.currentBlockHeight
}

func (cs *ChainStore) GetVotingWeight(hash common.Uint160) (int, error) {
	return 1, nil
}

func (cs *ChainStore) GetCurrentCachedHeaderHash() common.Uint256 {
	return cs.headerCache.GetCurrentCacheHeaderHash()
}

func (cs *ChainStore) GetCachedHeaderHash(height uint32) common.Uint256 {
	return cs.headerCache.GetCachedHeaderHashByHeight(height)
}

func (cs *ChainStore) GetCurrentCachedHeaderHeight() uint32 {
	return cs.headerCache.GetCurrentCachedHeight()
}

func (cs *ChainStore) persist(b *ledger.Block) error {
	batch := NewBatchStore(cs)
	errChan := make(chan error, 8)

	go func(b *ledger.Block, ch chan error) { ch <- batch.storeBlock(b) }(b, errChan)
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

	if err := batch.Commit(); err != nil {
		return err
	}

	cs.mu.Lock()
	cs.currentBlockHeight = b.Header.Height
	cs.currentBlockHash = b.Hash()
	cs.mu.Unlock()

	cs.headerCache.RemoveCachedHeader(cs.currentBlockHeight - 3)
	cs.headerCache.AddHeaderToCache(b.Header)
	return nil
}

func (cs *ChainStore) getUnspentByHeight(programHash common.Uint160, assetid common.Uint256, height uint32) ([]*tx.UTXOUnspent, error) {
	unspentsData, err := cs.db.Get(unspentUtxoKey(programHash, assetid, height))
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(unspentsData)
	listNum, err := serialization.ReadVarUint(r, 0)
	if err != nil {
		return nil, err
	}

	unspents := make([]*tx.UTXOUnspent, listNum)
	for i := 0; i < int(listNum); i++ {
		uu := new(tx.UTXOUnspent)
		if err := uu.Deserialize(r); err != nil {
			return nil, err
		}

		unspents[i] = uu
	}

	return unspents, nil
}

func (cs *ChainStore) RollbackBlock(bHash common.Uint256) *ledger.Block {
	return nil
}

func (cs *ChainStore) Dump() {
	fmt.Println("=================chainstore dump==================")
	cs.headerCache.Dump()
	fmt.Println("blockcache:")
	for hash, b := range cs.blockCache {
		fmt.Println(hash.ToHexString(), b)
	}

	fmt.Println("CurrentBlockHash", cs.currentBlockHash.ToHexString())
	fmt.Println("CurrentBlockHeight", cs.currentBlockHeight)
	fmt.Println("==================================================")
}
