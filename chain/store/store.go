package store

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/block"
	"github.com/nknorg/nkn/v2/chain/db"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/common/serialization"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/program"
	"github.com/nknorg/nkn/v2/transaction"
	"github.com/nknorg/nkn/v2/util/log"
)

const (
	beforeFastSync byte = 0x01
	beginFastSync  byte = 0x02
	postFastSync   byte = 0x03
)

type ChainStore struct {
	st db.IStore

	mu            sync.RWMutex
	blockCache    map[common.Uint256]*block.Block
	headerCache   *HeaderCache
	sigChainCache *SigChainCache
	States        *StateDB

	currentBlockHash   common.Uint256
	currentBlockHeight uint32
}

func NewLedgerStore() (*ChainStore, error) {
	st, err := db.NewLevelDBStore(config.Parameters.ChainDBPath)
	if err != nil {
		return nil, err
	}

	chain := &ChainStore{
		st:                 st,
		blockCache:         map[common.Uint256]*block.Block{},
		headerCache:        NewHeaderCache(),
		sigChainCache:      NewSigChainCache(),
		currentBlockHeight: 0,
		currentBlockHash:   common.EmptyUint256,
	}

	return chain, nil
}

func (cs *ChainStore) PrepareFastSync(fastSyncHeight uint32, fastSyncRootHash common.Uint256) error {
	err := cs.st.NewBatch()
	if err != nil {
		return err
	}

	err = cs.persistSyncRootHeight(fastSyncHeight)
	if err != nil {
		return err
	}

	err = cs.persistSyncStatus(beginFastSync)
	if err != nil {
		return err
	}

	err = cs.st.BatchPut(db.CurrentFastSyncRoot(), fastSyncRootHash.ToArray())
	if err != nil {
		return err
	}

	err = cs.st.BatchCommit()
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) FastSyncDone(syncRootHash common.Uint256, fastSyncHeight uint32) error {
	sdb, err := NewStateDB(syncRootHash, cs)
	if err != nil {
		return err
	}

	err = sdb.trie.TryTraverse(false)
	if err != nil {
		return err
	}

	hs := sdb.trie.Hash()
	if syncRootHash.CompareTo(hs) != 0 {
		return fmt.Errorf("state root not equal, should be: %v, we got: %v", syncRootHash.ToHexString(), hs.ToHexString())
	}
	cs.States = sdb

	refCount, err := sdb.trie.NewRefCounts(fastSyncHeight-1, fastSyncHeight-1)
	if err != nil {
		return err
	}

	err = refCount.PersistRefCountHeights()
	if err != nil {
		return err
	}

	err = refCount.PersistPrunedHeights()
	if err != nil {
		return err
	}

	err = refCount.PersistNeedReset()
	if err != nil {
		return err
	}

	err = refCount.Commit()
	if err != nil {
		return err
	}

	err = cs.st.NewBatch()
	if err != nil {
		return err
	}

	err = cs.persistCompactHeight(fastSyncHeight)
	if err != nil {
		return err
	}

	err = cs.persistSyncStatus(postFastSync)
	if err != nil {
		return err
	}

	err = cs.st.BatchPut(db.CurrentStateTrie(), syncRootHash.ToArray())
	if err != nil {
		return err
	}

	err = cs.st.BatchCommit()
	if err != nil {
		return err
	}
	return nil
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

func (cs *ChainStore) InitLedgerStoreWithGenesisBlock(genesisBlock *block.Block) (uint32, error) {
	version, err := cs.st.Get(db.VersionKey())
	if err != nil {
		version = []byte{0x00}
	}

	log.Info("database Version:", config.DBVersion)

	if version[0] != config.DBVersion {
		if err := cs.ResetDB(); err != nil {
			return 0, fmt.Errorf("InitLedgerStoreWithGenesisBlock, ResetDB error: %v", err)
		}

		root := common.EmptyUint256
		cs.States, err = NewStateDB(root, cs)
		if err != nil {
			return 0, err
		}

		if err := cs.persist(genesisBlock, false); err != nil {
			return 0, err
		}

		if err = cs.st.Put(db.VersionKey(), []byte{config.DBVersion}); err != nil {
			return 0, err
		}

		cs.headerCache.AddHeaderToCache(genesisBlock.Header)
		cs.currentBlockHash = genesisBlock.Hash()
		cs.currentBlockHeight = 0

		return 0, nil
	}

	if !cs.IsBlockInStore(genesisBlock.Hash()) {
		return 0, errors.New("genesisBlock is NOT in BlockStore")
	}

	if cs.currentBlockHash, cs.currentBlockHeight, err = cs.getCurrentBlockHashFromDB(); err != nil {
		return 0, err
	}

	var minHeight uint32
	if cs.currentBlockHeight >= config.Parameters.BlockHeaderCacheSize {
		minHeight = cs.currentBlockHeight - config.Parameters.BlockHeaderCacheSize + 1
	}
	// Use int to prevent uint underflow
	for height := int(cs.currentBlockHeight); height >= int(minHeight); height-- {
		header, err := cs.GetHeaderByHeight(uint32(height))
		if err != nil {
			return 0, err
		}
		cs.headerCache.AddHeaderToCache(header)
	}

	minHeight = 1
	if cs.currentBlockHeight >= config.Parameters.SigChainCacheSize {
		minHeight = cs.currentBlockHeight - config.Parameters.SigChainCacheSize + 1
	}
	// Use int to prevent uint underflow
	for height := int(cs.currentBlockHeight); height >= int(minHeight); height-- {
		header, err := cs.GetHeaderByHeight(uint32(height))
		if err != nil {
			return 0, err
		}
		err = cs.addHeaderSigChainToCache(header)
		if err != nil {
			return 0, err
		}
	}

	root, err := cs.GetCurrentBlockStateRoot()
	if err != nil {
		return 0, nil
	}

	log.Info("State root:", root.ToHexString())

	cs.States, err = NewStateDB(root, cs)
	if err != nil {
		return 0, err
	}

	switch config.Parameters.StatePruningMode {
	case "lowmem":
		err = cs.PruneStatesLowMemory(true)
	case "none":
		err = nil
	default:
		err = fmt.Errorf("unknown state pruning mode %v", config.Parameters.StatePruningMode)
	}

	if err != nil {
		return 0, err
	}

	return cs.currentBlockHeight, nil
}

func (cs *ChainStore) IsTxHashDuplicate(txhash common.Uint256) bool {
	if _, err := cs.st.Get(db.TransactionKey(txhash)); err != nil {
		return false
	}

	return true
}

func (cs *ChainStore) GetBlockHash(height uint32) (common.Uint256, error) {
	blockHash, err := cs.st.Get(db.BlockhashKey(height))
	if err != nil {
		return common.EmptyUint256, err
	}

	return common.Uint256ParseFromBytes(blockHash)
}

func (cs *ChainStore) GetBlockByHeight(height uint32) (*block.Block, error) {
	hash, err := cs.GetBlockHash(height)
	if err != nil {
		return nil, err
	}

	return cs.GetBlock(hash)
}

func (cs *ChainStore) GetHeader(hash common.Uint256) (*block.Header, error) {
	data, err := cs.st.Get(db.HeaderKey(hash))
	if err != nil {
		return nil, err
	}

	h := &block.Header{}
	dt, err := serialization.ReadVarBytes(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	err = h.Unmarshal(dt)
	if err != nil {
		return nil, err
	}

	return h, nil
}

func (cs *ChainStore) GetHeaderByHeight(height uint32) (*block.Header, error) {
	hash, err := cs.GetBlockHash(height)
	if err != nil {
		return nil, err
	}

	return cs.GetHeader(hash)
}

func (cs *ChainStore) GetTransaction(hash common.Uint256) (*transaction.Transaction, error) {
	t, _, err := cs.getTx(hash)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (cs *ChainStore) getTx(hash common.Uint256) (*transaction.Transaction, uint32, error) {
	value, err := cs.st.Get(db.TransactionKey(hash))
	if err != nil {
		return nil, 0, err
	}

	height := binary.LittleEndian.Uint32(value)
	value = value[4:]
	var txn transaction.Transaction
	if err := txn.Unmarshal(value); err != nil {
		return nil, height, err
	}

	return &txn, height, nil
}

func (cs *ChainStore) GetBlock(hash common.Uint256) (*block.Block, error) {
	bHash, err := cs.st.Get(db.HeaderKey(hash))
	if err != nil {
		return nil, err
	}

	b := new(block.Block)
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

func (cs *ChainStore) GetHeightByBlockHash(hash common.Uint256) (uint32, error) {
	header, err := cs.getHeaderWithCache(hash)
	if err == nil {
		return header.UnsignedHeader.Height, nil
	}

	header, err = cs.GetHeader(hash)
	if err != nil {
		return 0, err
	}

	return header.UnsignedHeader.Height, nil
}

func (cs *ChainStore) IsBlockInStore(hash common.Uint256) bool {
	if header, err := cs.GetHeader(hash); err != nil || header.UnsignedHeader.Height > cs.currentBlockHeight {
		return false
	}

	return true
}

func (cs *ChainStore) persistHeader(h *block.Header) error {
	err := cs.st.NewBatch()
	if err != nil {
		return err
	}

	headerHash := h.Hash()

	headerBuffer := bytes.NewBuffer(nil)
	headerBytes, err := h.Marshal()
	if err != nil {
		return err
	}

	err = serialization.WriteVarBytes(headerBuffer, headerBytes)
	if err != nil {
		return err
	}

	err = cs.st.BatchPut(db.HeaderKey(headerHash), headerBuffer.Bytes())
	if err != nil {
		return err
	}

	//batch put headerhash
	headerHashBuffer := bytes.NewBuffer(nil)
	_, err = headerHash.Serialize(headerHashBuffer)
	if err != nil {
		return err
	}

	err = cs.st.BatchPut(db.BlockhashKey(h.UnsignedHeader.Height), headerHashBuffer.Bytes())
	if err != nil {
		return err
	}

	err = cs.st.BatchCommit()
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) persist(b *block.Block, fastSync bool) error {
	err := cs.st.NewBatch()
	if err != nil {
		return err
	}

	headerHash := b.Hash()

	//batch put header
	headerBuffer := bytes.NewBuffer(nil)
	err = b.Trim(headerBuffer)
	if err != nil {
		return err
	}

	err = cs.st.BatchPut(db.HeaderKey(headerHash), headerBuffer.Bytes())
	if err != nil {
		return err
	}

	//batch put headerhash
	headerHashBuffer := bytes.NewBuffer(nil)
	_, err = headerHash.Serialize(headerHashBuffer)
	if err != nil {
		return err
	}

	err = cs.st.BatchPut(db.BlockhashKey(b.Header.UnsignedHeader.Height), headerHashBuffer.Bytes())
	if err != nil {
		return err
	}

	//batch put transactions
	for _, txn := range b.Transactions {
		buffer := make([]byte, 4)
		binary.LittleEndian.PutUint32(buffer[:], b.Header.UnsignedHeader.Height)
		dt, err := txn.Marshal()
		if err != nil {
			return err
		}

		buffer = append(buffer, dt...)

		if err := cs.st.BatchPut(db.TransactionKey(txn.Hash()), buffer); err != nil {
			return err
		}

		switch txn.UnsignedTx.Payload.Type {
		case pb.PayloadType_COINBASE_TYPE:
		case pb.PayloadType_SIG_CHAIN_TXN_TYPE:
		case pb.PayloadType_TRANSFER_ASSET_TYPE:
		case pb.PayloadType_ISSUE_ASSET_TYPE:
		case pb.PayloadType_REGISTER_NAME_TYPE:
		case pb.PayloadType_TRANSFER_NAME_TYPE:
		case pb.PayloadType_DELETE_NAME_TYPE:
		case pb.PayloadType_SUBSCRIBE_TYPE:
		case pb.PayloadType_UNSUBSCRIBE_TYPE:
		case pb.PayloadType_GENERATE_ID_TYPE:
		case pb.PayloadType_NANO_PAY_TYPE:
		default:
			return errors.New("unsupported transaction type")
		}
	}

	//StateRoot
	var states *StateDB
	var root common.Uint256
	if !fastSync {
		states, root, err = cs.generateStateRoot(context.Background(), b, b.Header.UnsignedHeader.Height != 0, true)
		if err != nil {
			return err
		}

		headerRoot, err := common.Uint256ParseFromBytes(b.Header.UnsignedHeader.StateRoot)
		if err != nil {
			return err
		}
		if ok := root.CompareTo(headerRoot); ok != 0 {
			return fmt.Errorf("state root not equal:%v, %v", root.ToHexString(), headerRoot.ToHexString())
		}

		err = cs.st.BatchPut(db.CurrentStateTrie(), root.ToArray())
		if err != nil {
			return err
		}
		// batch put donation
		if b.Header.UnsignedHeader.Height%uint32(config.RewardAdjustInterval) == 0 && !config.DonationNoDelay.GetValueAtHeight(b.Header.UnsignedHeader.Height) {
			donation, err := cs.CalcNextDonation_legacy(b.Header.UnsignedHeader.Height)
			if err != nil {
				return err
			}

			w := bytes.NewBuffer(nil)
			err = donation.Serialize(w)
			if err != nil {
				return err
			}

			if err := cs.st.BatchPut(db.DonationKey(b.Header.UnsignedHeader.Height), w.Bytes()); err != nil {
				return err
			}
		}
	}

	//batch put currentblockhash
	err = serialization.WriteUint32(headerHashBuffer, b.Header.UnsignedHeader.Height)
	if err != nil {
		return err
	}

	err = cs.st.BatchPut(db.CurrentBlockHashKey(), headerHashBuffer.Bytes())
	if err != nil {
		return err
	}

	err = cs.st.BatchCommit()
	if err != nil {
		return err
	}

	if !fastSync {
		cs.States = states
	}

	return nil
}

func (cs *ChainStore) SaveBlock(b *block.Block, pruning, fastSync bool) error {
	err := cs.persist(b, fastSync)
	if err != nil {
		log.Errorf("Persist block error: %v", err)
		return err
	}

	cs.mu.Lock()
	cs.currentBlockHeight = b.Header.UnsignedHeader.Height
	cs.currentBlockHash = b.Hash()
	cs.mu.Unlock()

	if cs.currentBlockHeight >= config.Parameters.SigChainCacheSize {
		hash := cs.headerCache.GetCachedHeaderHashByHeight(cs.currentBlockHeight - config.Parameters.SigChainCacheSize)
		header, err := cs.headerCache.GetCachedHeader(hash)
		if err == nil {
			cs.sigChainCache.RemoveCachedSigChain(header.UnsignedHeader.WinnerHash)
		}
	}

	if cs.currentBlockHeight >= config.Parameters.BlockHeaderCacheSize {
		cs.headerCache.RemoveCachedHeader(cs.currentBlockHeight - config.Parameters.BlockHeaderCacheSize)
	}

	cs.headerCache.AddHeaderToCache(b.Header)

	err = cs.addHeaderSigChainToCache(b.Header)
	if err != nil {
		log.Errorf("Add header sigchain to cache: %v", err)
		return err
	}

	if pruning && !fastSync {
		switch config.Parameters.StatePruningMode {
		case "lowmem":
			err = cs.PruneStatesLowMemory(false)
			if err != nil {
				log.Errorf("Pruning error: %v", err)
				return err
			}
		}
	}

	return nil
}

func (cs *ChainStore) GetCurrentBlockHash() common.Uint256 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.currentBlockHash
}

func (cs *ChainStore) GetHeight() uint32 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.currentBlockHeight
}

func (cs *ChainStore) AddHeader(header *block.Header) error {
	err := cs.persistHeader(header)
	if err != nil {
		return err
	}

	if header.UnsignedHeader.Height >= config.Parameters.BlockHeaderCacheSize {
		cs.headerCache.RemoveCachedHeader(header.UnsignedHeader.Height - config.Parameters.BlockHeaderCacheSize)
	}

	cs.headerCache.AddHeaderToCache(header)

	return nil
}

func (cs *ChainStore) GetHeaderHeight() uint32 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.headerCache.GetCurrentCachedHeight()
}

func (cs *ChainStore) GetCurrentHeaderHash() common.Uint256 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.headerCache.GetCurrentCacheHeaderHash()
}

func (cs *ChainStore) GetHeaderHashByHeight(height uint32) common.Uint256 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.headerCache.GetCachedHeaderHashByHeight(height)
}

func (cs *ChainStore) GetHeaderWithCache(hash common.Uint256) (*block.Header, error) {
	return cs.headerCache.GetCachedHeader(hash)
}

func (cs *ChainStore) getHeaderWithCache(hash common.Uint256) (*block.Header, error) {
	return cs.headerCache.GetCachedHeader(hash)
}

func (cs *ChainStore) GetSigChainWithCache(hash common.Uint256) (*pb.SigChain, error) {
	return cs.sigChainCache.GetCachedSigChain(hash)
}

func (cs *ChainStore) addHeaderSigChainToCache(header *block.Header) error {
	hash, err := common.Uint256ParseFromBytes(header.UnsignedHeader.WinnerHash)
	if err != nil {
		return err
	}

	if hash == common.EmptyUint256 {
		return nil
	}

	txn, err := cs.GetTransaction(hash)
	if err != nil {
		return fmt.Errorf("sigchain txn %s not found in ledger", hash)
	}

	payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return err
	}

	sc := &pb.SigChain{}
	err = proto.Unmarshal(payload.(*pb.SigChainTxn).SigChain, sc)
	if err != nil {
		return err
	}

	cs.sigChainCache.AddSigChainToCache(hash, sc)

	return nil
}

func (cs *ChainStore) IsDoubleSpend(tx *transaction.Transaction) bool {
	return false
}

func (cs *ChainStore) GetCurrentBlockHashFromDB() (common.Uint256, uint32, error) {
	return cs.getCurrentBlockHashFromDB()
}

func (cs *ChainStore) getCurrentBlockHashFromDB() (common.Uint256, uint32, error) {
	data, err := cs.st.Get(db.CurrentBlockHashKey())
	if err != nil {
		return common.EmptyUint256, 0, err
	}

	var blockHash common.Uint256
	r := bytes.NewReader(data)
	blockHash.Deserialize(r)
	currentHeight, err := serialization.ReadUint32(r)
	return blockHash, currentHeight, err
}

func (cs *ChainStore) GetCurrentBlockStateRoot() (common.Uint256, error) {
	currentState, err := cs.st.Get(db.CurrentStateTrie())
	if err != nil {
		return common.EmptyUint256, err
	}

	hash, err := common.Uint256ParseFromBytes(currentState)
	if err != nil {
		return common.EmptyUint256, err
	}

	return hash, nil
}

func (cs *ChainStore) GetFastSyncStateRoot() (common.Uint256, error) {
	currentState, err := cs.st.Get(db.CurrentFastSyncRoot())
	if err != nil {
		return common.EmptyUint256, err
	}

	hash, err := common.Uint256ParseFromBytes(currentState)
	if err != nil {
		return common.EmptyUint256, err
	}

	return hash, nil
}

func (cs *ChainStore) GetDatabase() db.IStore {
	return cs.st
}

func (cs *ChainStore) GetBalance(addr common.Uint160) common.Fixed64 {
	return cs.States.GetBalance(config.NKNAssetID, addr)
}

func (cs *ChainStore) GetBalanceByAssetID(addr common.Uint160, assetID common.Uint256) common.Fixed64 {
	return cs.States.GetBalance(assetID, addr)
}

func (cs *ChainStore) GetNonce(addr common.Uint160) uint64 {
	return cs.States.GetNonce(addr)
}

func (cs *ChainStore) GetID(publicKey []byte, height uint32) ([]byte, error) {
	programHash, err := program.CreateProgramHash(publicKey)
	if err != nil {
		return nil, fmt.Errorf("GetID error: %v", err)
	}

	return cs.States.GetID(programHash, height), nil
}

func (cs *ChainStore) GetIDVersion(publicKey []byte) ([]byte, byte, error) {
	programHash, err := program.CreateProgramHash(publicKey)
	if err != nil {
		return nil, 0, fmt.Errorf("GetID error: %v", err)
	}

	id, version := cs.States.GetIDVersion(programHash)
	return id, version, nil
}

func (cs *ChainStore) GetNanoPay(addr common.Uint160, recipient common.Uint160, nonce uint64) (common.Fixed64, uint32, error) {
	return cs.States.GetNanoPay(addr, recipient, nonce)
}

type Donation struct {
	Height uint32
	Amount common.Fixed64
}

func NewDonation(height uint32, amount common.Fixed64) *Donation {
	return &Donation{
		Height: height,
		Amount: amount,
	}
}

func (d *Donation) Serialize(w io.Writer) error {
	err := serialization.WriteUint32(w, d.Height)
	if err != nil {
		return err
	}

	err = d.Amount.Serialize(w)
	if err != nil {
		return err
	}

	return nil
}

func (d *Donation) Deserialize(r io.Reader) error {
	var err error
	d.Height, err = serialization.ReadUint32(r)
	if err != nil {
		return err
	}

	err = d.Amount.Deserialize(r)
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) GetDonation() (common.Fixed64, error) {
	donation, err := cs.getDonation()
	if err != nil {
		return common.Fixed64(0), err
	}
	return donation.Amount, nil
}

func (cs *ChainStore) getDonation() (*Donation, error) {
	if config.DonationNoDelay.GetValueAtHeight(cs.currentBlockHeight + 1) {
		donationProgramhash, _ := common.ToScriptHash(config.DonationAddress)
		totalDonationAmount := cs.GetBalance(donationProgramhash)
		donationAmount := totalDonationAmount * 11 / 25000000
		d := NewDonation(cs.currentBlockHeight+1, donationAmount)
		return d, nil
	}
	currentDonationHeight := cs.currentBlockHeight / uint32(config.RewardAdjustInterval) * uint32(config.RewardAdjustInterval)
	data, err := cs.st.Get(db.DonationKey(currentDonationHeight))
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(data)
	donation := new(Donation)
	err = donation.Deserialize(r)
	if err != nil {
		return nil, err
	}

	return donation, nil
}

func (cs *ChainStore) CalcNextDonation_legacy(height uint32) (*Donation, error) {
	if height == 0 {
		return NewDonation(0, 0), nil
	}

	lastDonation, err := cs.getDonation()
	if err != nil {
		return nil, err
	}

	if lastDonation.Height+uint32(config.RewardAdjustInterval) != height {
		return nil, errors.New("invalid height to update donation")
	}

	donationAddress, err := common.ToScriptHash(config.DonationAddress)
	if err != nil {
		return nil, err
	}
	account := cs.States.GetOrNewAccount(donationAddress)
	amount := account.GetBalance(config.NKNAssetID)
	donation := amount * config.DonationAdjustDividendFactor / config.DonationAdjustDivisorFactor
	donationPerBlock := int64(donation) / int64(config.RewardAdjustInterval)

	d := NewDonation(height, common.Fixed64(donationPerBlock))

	return d, nil
}

func (cs *ChainStore) GetStateRoots(fromHeight, toHeight uint32) ([]common.Uint256, error) {
	if toHeight < fromHeight {
		return nil, fmt.Errorf("toHeight(%v) is less than fromHeight(%v)\n", toHeight, fromHeight)
	}
	roots := make([]common.Uint256, 0, toHeight-fromHeight+1)

	for i := fromHeight; i <= toHeight; i++ {
		header, err := cs.GetHeaderByHeight(i)
		if err != nil {
			return nil, err
		}
		stateRoot, err := common.Uint256ParseFromBytes(header.UnsignedHeader.StateRoot)
		if err != nil {
			return nil, err
		}

		roots = append(roots, stateRoot)
	}

	return roots, nil
}

func (cs *ChainStore) GetPruningStartHeight() (uint32, uint32) {
	return cs.getPruningStartHeight()
}

func (cs *ChainStore) getPruningStartHeight() (uint32, uint32) {
	var pruningStartHeight, refCountStartHeight uint32

	heightBuffer, err := cs.st.Get(db.TrieRefCountHeightKey())
	if err != nil {
		log.Info("get height of trie counted error:", err)
		refCountStartHeight = 0
	} else {
		refCountStartHeight = binary.LittleEndian.Uint32(heightBuffer) + 1
	}

	heightBuffer, err = cs.st.Get(db.TriePrunedHeightKey())
	if err != nil {
		log.Info("get height of trie pruned error:", err)
		pruningStartHeight = 0
	} else {
		pruningStartHeight = binary.LittleEndian.Uint32(heightBuffer) + 1
	}

	return refCountStartHeight, pruningStartHeight
}

func (cs *ChainStore) getCompactHeight() uint32 {
	heightBuffer, err := cs.st.Get(db.TrieCompactHeightKey())
	if err != nil {
		log.Info("get compact height error:", err)
		return 0
	}
	return binary.LittleEndian.Uint32(heightBuffer)
}

func (cs *ChainStore) persistCompactHeight(height uint32) error {
	heightBuffer := make([]byte, 4)
	binary.LittleEndian.PutUint32(heightBuffer[:], height)
	return cs.st.BatchPut(db.TrieCompactHeightKey(), heightBuffer)
}

func (cs *ChainStore) persistSyncRootHeight(height uint32) error {
	heightBuffer := make([]byte, 4)
	binary.LittleEndian.PutUint32(heightBuffer[:], height)
	return cs.st.BatchPut(db.TrieFastSyncRootHeightKey(), heightBuffer)
}

func (cs *ChainStore) persistSyncStatus(status byte) error {
	return cs.st.BatchPut(db.TrieFastSyncStatusKey(), []byte{status})
}

func (cs *ChainStore) GetSyncRootHeight() (uint32, error) {
	return cs.getSyncRootHeight()
}

func (cs *ChainStore) getSyncRootHeight() (uint32, error) {
	heightBuffer, err := cs.st.Get(db.TrieFastSyncRootHeightKey())
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(heightBuffer), nil
}

func (cs *ChainStore) getFastSyncStatus() byte {
	status, err := cs.st.Get(db.TrieFastSyncStatusKey())
	if err != nil {
		log.Info("no previous fast sync status found")
		return beforeFastSync
	}
	return status[0]
}

func (cs *ChainStore) ShouldFastSync(syncStopHeight uint32) bool {
	status := cs.getFastSyncStatus()
	switch status {
	case beforeFastSync:
		if syncStopHeight <= config.Parameters.RecentStateCount {
			return false
		}
		if cs.GetHeight() == 0 {
			return true
		}
	case beginFastSync:
		return true
	case postFastSync:
		return false
	}
	return false
}

func (cs *ChainStore) PruneStatesLowMemory(full bool) error {
	state, err := NewStateDB(common.EmptyUint256, cs)
	if err != nil {
		return err
	}

	return state.PruneStatesLowMemory(full)
}

// PruneStates is not in use due to high memory usage. Use PruneStatesLowMemory
// instead.
func (cs *ChainStore) PruneStates() error {
	state, err := NewStateDB(common.EmptyUint256, cs)
	if err != nil {
		return err
	}

	return state.PruneStates()
}

// SequentialPrune is not in use due to high memory usage. Use
// PruneStatesLowMemory instead.
func (cs *ChainStore) SequentialPrune() error {
	state, err := NewStateDB(common.EmptyUint256, cs)
	if err != nil {
		return err
	}

	return state.SequentialPrune()
}

func (cs *ChainStore) TrieTraverse(needPrint bool) error {
	_, currentHeight, err := cs.getCurrentBlockHashFromDB()
	if err != nil {
		return err
	}

	roots, err := cs.GetStateRoots(currentHeight, currentHeight)
	if err != nil {
		return err
	}

	states, err := NewStateDB(roots[0], cs)
	if err != nil {
		return err
	}

	return states.TrieTraverse(needPrint)
}

func (cs *ChainStore) VerifyState() error {
	state, err := NewStateDB(common.EmptyUint256, cs)
	if err != nil {
		return err
	}

	return state.VerifyState()
}
