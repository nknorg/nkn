package db

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/signature"
	"github.com/nknorg/nkn/types"
	"github.com/nknorg/nkn/util/log"
)

type ChainStore struct {
	st IStore

	mu          sync.RWMutex
	blockCache  map[Uint256]*types.Block
	headerCache *HeaderCache
	States      *StateDB

	currentBlockHash   Uint256
	currentBlockHeight uint32
}

func NewLedgerStore() (core.ILedgerStore, error) {
	st, err := NewLevelDBStore("Chain")
	if err != nil {
		return nil, err
	}

	chain := &ChainStore{
		st:                 st,
		blockCache:         map[Uint256]*types.Block{},
		headerCache:        NewHeaderCache(),
		currentBlockHeight: 0,
		currentBlockHash:   EmptyUint256,
	}

	return chain, nil
}

func (cs *ChainStore) Close() {
	cs.st.Close()
}

func (cs *ChainStore) ResetDB() error {
	cs.st.NewBatch()
	iter := cs.st.NewIterator(nil)
	for iter.Next() {
		cs.st.BatchDelete(iter.Key())
	}
	iter.Release()

	return cs.st.BatchCommit()
}

func (cs *ChainStore) InitLedgerStoreWithGenesisBlock(genesisBlock *types.Block) (uint32, error) {
	version, err := cs.st.Get(versionKey())
	if err != nil {
		version = []byte{0x00}
	}

	if version[0] == 0x01 {
		if !cs.IsBlockInStore(genesisBlock.Hash()) {
			return 0, errors.New("genesisBlock is NOT in BlockStore.")
		}

		if cs.currentBlockHash, cs.currentBlockHeight, err = cs.getCurrentBlockHashFromDB(); err != nil {
			return 0, err
		}
		currentHeader, _ := cs.GetHeader(cs.currentBlockHash)

		cs.headerCache.AddHeaderToCache(currentHeader)

		root := cs.GetCurrentBlockStateRoot()
		fmt.Println("---------", root)
		cs.States, _ = NewStateDB(root, NewTrieStore(cs.GetDatabase()))

		return cs.currentBlockHeight, nil

	} else {
		if err := cs.ResetDB(); err != nil {
			return 0, fmt.Errorf("InitLedgerStoreWithGenesisBlock, ResetDB error: %v", err)
		}

		root := EmptyUint256
		cs.States, _ = NewStateDB(root, NewTrieStore(cs.GetDatabase()))

		if err := cs.persist(genesisBlock); err != nil {
			return 0, err
		}

		// put version to db
		if err = cs.st.Put(versionKey(), []byte{0x01}); err != nil {
			return 0, err
		}

		cs.headerCache.AddHeaderToCache(genesisBlock.Header)
		cs.currentBlockHash = genesisBlock.Hash()
		cs.currentBlockHeight = 0

		return 0, nil
	}
}

func (cs *ChainStore) IsTxHashDuplicate(txhash Uint256) bool {
	if _, err := cs.st.Get(transactionKey(txhash)); err != nil {
		return false
	}

	return true
}

func (cs *ChainStore) GetBlockHash(height uint32) (Uint256, error) {
	blockHash, err := cs.st.Get(blockhashKey(height))
	if err != nil {
		return EmptyUint256, err
	}

	return Uint256ParseFromBytes(blockHash)
}

func (cs *ChainStore) GetBlockByHeight(height uint32) (*types.Block, error) {
	hash, err := cs.GetBlockHash(height)
	if err != nil {
		return nil, err
	}

	return cs.GetBlock(hash)
}

func (cs *ChainStore) GetHeader(hash Uint256) (*types.Header, error) {
	data, err := cs.st.Get(headerKey(hash))
	if err != nil {
		return nil, err
	}

	h := new(types.Header)
	err = h.Deserialize(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return h, nil
}

func (cs *ChainStore) GetHeaderByHeight(height uint32) (*types.Header, error) {
	hash, err := cs.GetBlockHash(height)
	if err != nil {
		return nil, err
	}

	return cs.GetHeader(hash)
}

func (cs *ChainStore) GetTransaction(hash Uint256) (*types.Transaction, error) {
	t, _, err := cs.getTx(hash)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (cs *ChainStore) getTx(hash Uint256) (*types.Transaction, uint32, error) {
	value, err := cs.st.Get(transactionKey(hash))
	if err != nil {
		return nil, 0, err
	}

	r := bytes.NewReader(value)
	height, err := serialization.ReadUint32(r)
	if err != nil {
		return nil, 0, err
	}

	var txn types.Transaction
	if err := txn.Deserialize(r); err != nil {
		return nil, height, err
	}

	return &txn, height, nil
}

func (cs *ChainStore) GetBlock(hash Uint256) (*types.Block, error) {
	bHash, err := cs.st.Get(headerKey(hash))
	if err != nil {
		return nil, err
	}

	b := new(types.Block)
	if err = b.FromTrimmedData(bytes.NewReader(bHash)); err != nil {
		return nil, err
	}

	for i := 0; i < len(b.Transactions); i++ {
		if b.Transactions[i], _, err = cs.getTx(b.Transactions[i].Hash()); err != nil {
			return nil, err
		}
	}

	return b, nil
}

func (cs *ChainStore) GetHeightByBlockHash(hash Uint256) (uint32, error) {
	header, err := cs.getHeaderWithCache(hash)
	if err == nil {
		return header.UnsignedHeader.Height, nil
	}

	block, err := cs.GetBlock(hash)
	if err != nil {
		return 0, err
	}

	return block.Header.UnsignedHeader.Height, nil
}

func (cs *ChainStore) IsBlockInStore(hash Uint256) bool {
	if header, err := cs.GetHeader(hash); err != nil || header.UnsignedHeader.Height > cs.currentBlockHeight {
		return false
	}

	return true
}

func (cs *ChainStore) persist(b *types.Block) error {
	cs.st.NewBatch()

	headerHash := b.Hash()

	//batch put header
	headerBuffer := bytes.NewBuffer(nil)
	b.Trim(headerBuffer)
	if err := cs.st.BatchPut(headerKey(headerHash), headerBuffer.Bytes()); err != nil {
		return err
	}

	//batch put headerhash
	headerHashBuffer := bytes.NewBuffer(nil)
	headerHash.Serialize(headerHashBuffer)
	if err := cs.st.BatchPut(blockhashKey(b.Header.UnsignedHeader.Height), headerHashBuffer.Bytes()); err != nil {
		return err
	}

	//batch put transactions
	for _, txn := range b.Transactions {
		w := bytes.NewBuffer(nil)
		serialization.WriteUint32(w, b.Header.UnsignedHeader.Height)
		txn.Serialize(w)

		if err := cs.st.BatchPut(transactionKey(txn.Hash()), w.Bytes()); err != nil {
			return err
		}

		switch txn.UnsignedTx.Payload.Type {
		case types.CoinbaseType:
			pl, err := types.Unpack(txn.UnsignedTx.Payload)
			if err != nil {
				return err
			}
			coinbase := pl.(*types.Coinbase)
			acc := cs.States.GetOrNewAccount(BytesToUint160(coinbase.Recipient))
			amount := acc.GetBalance()
			acc.SetBalance(amount + Fixed64(coinbase.Amount))
			cs.States.setAccount(BytesToUint160(coinbase.Recipient), acc)
		case types.TransferAssetType:
			pl, err := types.Unpack(txn.UnsignedTx.Payload)
			if err != nil {
				return err
			}
			transfer := pl.(*types.TransferAsset)
			accSender := cs.States.GetOrNewAccount(BytesToUint160(transfer.Sender))
			amountSender := accSender.GetBalance()
			accSender.SetBalance(amountSender - Fixed64(transfer.Amount))
			cs.States.setAccount(BytesToUint160(transfer.Sender), accSender)

			accRecipient := cs.States.GetOrNewAccount(BytesToUint160(transfer.Recipient))
			amountRecipient := accRecipient.GetBalance()
			accRecipient.SetBalance(amountRecipient + Fixed64(transfer.Amount))
			cs.States.setAccount(BytesToUint160(transfer.Recipient), accRecipient)
		case types.RegisterNameType:
			pl, err := types.Unpack(txn.UnsignedTx.Payload)
			if err != nil {
				return err
			}
			registerNamePayload := pl.(*types.RegisterName)
			err = cs.SaveName(registerNamePayload.Registrant, registerNamePayload.Name)
			if err != nil {
				return err
			}
		case types.DeleteNameType:
			pl, err := types.Unpack(txn.UnsignedTx.Payload)
			if err != nil {
				return err
			}
			deleteNamePayload := pl.(*types.DeleteName)
			err = cs.DeleteName(deleteNamePayload.Registrant)
			if err != nil {
				return err
			}
		case types.SubscribeType:
			pl, err := types.Unpack(txn.UnsignedTx.Payload)
			if err != nil {
				return err
			}
			subscribePayload := pl.(*types.Subscribe)
			err = cs.Subscribe(subscribePayload.Subscriber, subscribePayload.Identifier, subscribePayload.Topic, subscribePayload.Bucket, subscribePayload.Duration, subscribePayload.Meta, b.Header.UnsignedHeader.Height)
			if err != nil {
				return err
			}

		}
	}

	expiredKeys := cs.GetExpiredKeys(b.Header.UnsignedHeader.Height)
	for i := 0; i < len(expiredKeys); i++ {
		err := cs.RemoveExpiredKey(expiredKeys[i])
		if err != nil {
			return err
		}
	}

	//StateRoot
	root, err := cs.States.CommitTo(true)
	if err != nil {
		return err
	}

	err = cs.st.BatchPut(currentStateTrie(), root.ToArray())
	if err != nil {
		return err
	}

	//batch put currentblockhash
	serialization.WriteUint32(headerHashBuffer, b.Header.UnsignedHeader.Height)
	err = cs.st.BatchPut(currentBlockHashKey(), headerHashBuffer.Bytes())
	if err != nil {
		return err
	}

	return cs.st.BatchCommit()
}

func (cs *ChainStore) SaveBlock(b *types.Block, ledger *core.Ledger, fastAdd bool) error {
	if err := cs.persist(b); err != nil {
		log.Error("error to persist block:", err.Error())
		return err
	}

	cs.mu.Lock()
	cs.currentBlockHeight = b.Header.UnsignedHeader.Height
	cs.currentBlockHash = b.Hash()
	cs.mu.Unlock()

	if cs.currentBlockHeight > 3 {
		cs.headerCache.RemoveCachedHeader(cs.currentBlockHeight - 3)
	}
	cs.headerCache.AddHeaderToCache(b.Header)

	ledger.Blockchain.BlockHeight = cs.GetHeight()
	ledger.Blockchain.BCEvents.Notify(events.EventBlockPersistCompleted, b)

	log.Infof("# current block height: %d, block hash: %x", cs.currentBlockHeight, cs.currentBlockHash.ToArrayReverse())

	return nil
}

func (cs *ChainStore) GetCurrentBlockHash() Uint256 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.currentBlockHash
}

func (cs *ChainStore) GetHeight() uint32 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.currentBlockHeight
}

func (cs *ChainStore) verifyHeader(header *types.Header) bool {
	prevHash, _ := Uint256ParseFromBytes(header.UnsignedHeader.PrevBlockHash)
	prevHeader, err := cs.getHeaderWithCache(prevHash)
	if err != nil || prevHeader == nil {
		log.Error("[verifyHeader] failed, not found prevHeader.")
		return false
	}

	if prevHeader.UnsignedHeader.Height+1 != header.UnsignedHeader.Height {
		log.Error("[verifyHeader] failed, prevHeader.Height + 1 != header.Height")
		return false
	}

	if prevHeader.UnsignedHeader.Timestamp >= header.UnsignedHeader.Timestamp {
		log.Error("[verifyHeader] failed, prevHeader.Timestamp >= header.Timestamp")
		return false
	}

	flag, err := signature.VerifySignableData(header)
	if flag == false || err != nil {
		log.Error("[verifyHeader] failed, VerifySignableData failed.")
		log.Error(err)
		return false
	}

	return true
}

func (cs *ChainStore) AddHeaders(headers []*types.Header) error {
	sort.Slice(headers, func(i, j int) bool {
		return headers[i].UnsignedHeader.Height < headers[j].UnsignedHeader.Height
	})

	for i := 0; i < len(headers); i++ {
		//if headers[i].Height != cs.GetHeaderHeight()+1 {
		//	return errors.New("header height error.")
		//}

		//if !cs.verifyHeader(headers[i]) {
		//	return errors.New("header verify error.")
		//}

		cs.headerCache.AddHeaderToCache(headers[i])
	}

	return nil

}

func (cs *ChainStore) GetHeaderHeight() uint32 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.headerCache.GetCurrentCachedHeight()
}

func (cs *ChainStore) GetCurrentHeaderHash() Uint256 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.headerCache.GetCurrentCacheHeaderHash()
}

func (cs *ChainStore) GetHeaderHashByHeight(height uint32) Uint256 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.headerCache.GetCachedHeaderHashByHeight(height)
}

func (cs *ChainStore) getHeaderWithCache(hash Uint256) (*types.Header, error) {
	return cs.headerCache.GetCachedHeader(hash)
}

func (cs *ChainStore) IsDoubleSpend(tx *types.Transaction) bool {
	//TODO implament
	return false
}

func (cs *ChainStore) getCurrentBlockHashFromDB() (Uint256, uint32, error) {
	data, err := cs.st.Get(currentBlockHashKey())
	if err != nil {
		return EmptyUint256, 0, err
	}

	var blockHash Uint256
	r := bytes.NewReader(data)
	blockHash.Deserialize(r)
	currentHeight, err := serialization.ReadUint32(r)
	return blockHash, currentHeight, err
}

func (cs *ChainStore) GetCurrentBlockStateRoot() Uint256 {
	currentState, _ := cs.st.Get(currentStateTrie())
	hash, _ := Uint256ParseFromBytes(currentState)

	return hash
}

func (cs *ChainStore) GetDatabase() IStore {
	return cs.st
}

func (cs *ChainStore) GetStateRootHash() Uint256 {
	return cs.States.IntermediateRoot(false)
}

func (cs *ChainStore) GetBalance(addr Uint160) Fixed64 {
	acc, err := cs.States.getAccount(addr)
	if err != nil {
		return Fixed64(0)
	}

	return acc.GetBalance()
}
