package store

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/chain/db"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/program"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

type ChainStore struct {
	st db.IStore

	mu          sync.RWMutex
	blockCache  map[Uint256]*block.Block
	headerCache *HeaderCache
	States      *StateDB

	currentBlockHash   Uint256
	currentBlockHeight uint32
}

func NewLedgerStore() (*ChainStore, error) {
	st, err := db.NewLevelDBStore(config.Parameters.ChainDBPath)
	if err != nil {
		return nil, err
	}

	chain := &ChainStore{
		st:                 st,
		blockCache:         map[Uint256]*block.Block{},
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

func (cs *ChainStore) InitLedgerStoreWithGenesisBlock(genesisBlock *block.Block) (uint32, error) {
	version, err := cs.st.Get(db.VersionKey())
	if err != nil {
		version = []byte{0x00}
	}

	log.Info("database Version:", config.DBVersion)
	if version[0] == config.DBVersion {
		if !cs.IsBlockInStore(genesisBlock.Hash()) {
			return 0, errors.New("genesisBlock is NOT in BlockStore.")
		}

		if cs.currentBlockHash, cs.currentBlockHeight, err = cs.getCurrentBlockHashFromDB(); err != nil {
			return 0, err
		}
		currentHeader, err := cs.GetHeader(cs.currentBlockHash)
		if err != nil {
			return 0, err
		}

		cs.headerCache.AddHeaderToCache(currentHeader)

		root, err := cs.GetCurrentBlockStateRoot()
		if err != nil {
			return 0, nil
		}

		log.Info("state root:", root.ToHexString())
		cs.States, err = NewStateDB(root, cs)
		if err != nil {
			return 0, err
		}

		switch config.Parameters.StatePruningMode {
		case "lowmem":
			err = cs.PruneStatesLowMemory()
		case "none":
			err = nil
		default:
			return 0, fmt.Errorf("unknown state pruning mode %v", config.Parameters.StatePruningMode)
		}

		if err != nil {
			return 0, err
		}

		return cs.currentBlockHeight, nil
	} else {
		if err := cs.ResetDB(); err != nil {
			return 0, fmt.Errorf("InitLedgerStoreWithGenesisBlock, ResetDB error: %v", err)
		}

		root := EmptyUint256
		cs.States, err = NewStateDB(root, cs)
		if err != nil {
			return 0, err
		}

		if err := cs.persist(genesisBlock); err != nil {
			return 0, err
		}

		// put version to db
		if err = cs.st.Put(db.VersionKey(), []byte{config.DBVersion}); err != nil {
			return 0, err
		}

		cs.headerCache.AddHeaderToCache(genesisBlock.Header)
		cs.currentBlockHash = genesisBlock.Hash()
		cs.currentBlockHeight = 0

		return 0, nil
	}
}

func (cs *ChainStore) IsTxHashDuplicate(txhash Uint256) bool {
	if _, err := cs.st.Get(db.TransactionKey(txhash)); err != nil {
		return false
	}

	return true
}

func (cs *ChainStore) GetBlockHash(height uint32) (Uint256, error) {
	blockHash, err := cs.st.Get(db.BlockhashKey(height))
	if err != nil {
		return EmptyUint256, err
	}

	return Uint256ParseFromBytes(blockHash)
}

func (cs *ChainStore) GetBlockByHeight(height uint32) (*block.Block, error) {
	hash, err := cs.GetBlockHash(height)
	if err != nil {
		return nil, err
	}

	return cs.GetBlock(hash)
}

func (cs *ChainStore) GetHeader(hash Uint256) (*block.Header, error) {
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

func (cs *ChainStore) GetTransaction(hash Uint256) (*transaction.Transaction, error) {
	t, _, err := cs.getTx(hash)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (cs *ChainStore) getTx(hash Uint256) (*transaction.Transaction, uint32, error) {
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

func (cs *ChainStore) GetBlock(hash Uint256) (*block.Block, error) {
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

func (cs *ChainStore) persist(b *block.Block) error {
	cs.st.NewBatch()

	headerHash := b.Hash()

	//batch put header
	headerBuffer := bytes.NewBuffer(nil)
	b.Trim(headerBuffer)
	if err := cs.st.BatchPut(db.HeaderKey(headerHash), headerBuffer.Bytes()); err != nil {
		return err
	}

	//batch put headerhash
	headerHashBuffer := bytes.NewBuffer(nil)
	headerHash.Serialize(headerHashBuffer)
	if err := cs.st.BatchPut(db.BlockhashKey(b.Header.UnsignedHeader.Height), headerHashBuffer.Bytes()); err != nil {
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
		case pb.COINBASE_TYPE:
		case pb.SIG_CHAIN_TXN_TYPE:
		case pb.TRANSFER_ASSET_TYPE:
		case pb.ISSUE_ASSET_TYPE:
		case pb.REGISTER_NAME_TYPE:
		case pb.DELETE_NAME_TYPE:
		case pb.SUBSCRIBE_TYPE:
		case pb.UNSUBSCRIBE_TYPE:
		case pb.GENERATE_ID_TYPE:
		case pb.NANO_PAY_TYPE:
		default:
			return errors.New("unsupported transaction type")
		}
	}

	//StateRoot
	states, root, err := cs.generateStateRoot(context.Background(), b, b.Header.UnsignedHeader.Height != 0, true)
	if err != nil {
		return err
	}

	headerRoot, err := Uint256ParseFromBytes(b.Header.UnsignedHeader.StateRoot)
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
	if b.Header.UnsignedHeader.Height%uint32(config.RewardAdjustInterval) == 0 {
		donation, err := cs.CalcNextDonation(b.Header.UnsignedHeader.Height)
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

	//batch put currentblockhash
	serialization.WriteUint32(headerHashBuffer, b.Header.UnsignedHeader.Height)
	err = cs.st.BatchPut(db.CurrentBlockHashKey(), headerHashBuffer.Bytes())
	if err != nil {
		return err
	}

	err = cs.st.BatchCommit()
	if err != nil {
		return err
	}

	cs.States = states

	return nil
}

func (cs *ChainStore) SaveBlock(b *block.Block, fastAdd bool) error {
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

func (cs *ChainStore) AddHeader(header *block.Header) error {
	cs.headerCache.AddHeaderToCache(header)

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

func (cs *ChainStore) GetHeaderWithCache(hash Uint256) (*block.Header, error) {
	return cs.headerCache.GetCachedHeader(hash)
}

func (cs *ChainStore) getHeaderWithCache(hash Uint256) (*block.Header, error) {
	return cs.headerCache.GetCachedHeader(hash)
}

func (cs *ChainStore) IsDoubleSpend(tx *transaction.Transaction) bool {
	return false
}

func (cs *ChainStore) GetCurrentBlockHashFromDB() (Uint256, uint32, error) {
	return cs.getCurrentBlockHashFromDB()
}

func (cs *ChainStore) getCurrentBlockHashFromDB() (Uint256, uint32, error) {
	data, err := cs.st.Get(db.CurrentBlockHashKey())
	if err != nil {
		return EmptyUint256, 0, err
	}

	var blockHash Uint256
	r := bytes.NewReader(data)
	blockHash.Deserialize(r)
	currentHeight, err := serialization.ReadUint32(r)
	return blockHash, currentHeight, err
}

func (cs *ChainStore) GetCurrentBlockStateRoot() (Uint256, error) {
	currentState, err := cs.st.Get(db.CurrentStateTrie())
	if err != nil {
		return EmptyUint256, err
	}

	hash, err := Uint256ParseFromBytes(currentState)
	if err != nil {
		return EmptyUint256, err
	}

	return hash, nil
}

func (cs *ChainStore) GetDatabase() db.IStore {
	return cs.st
}

func (cs *ChainStore) GetBalance(addr Uint160) Fixed64 {
	return cs.States.GetBalance(config.NKNAssetID, addr)
}

func (cs *ChainStore) GetBalanceByAssetID(addr Uint160, assetID Uint256) Fixed64 {
	return cs.States.GetBalance(assetID, addr)
}

func (cs *ChainStore) GetNonce(addr Uint160) uint64 {
	return cs.States.GetNonce(addr)
}

func (cs *ChainStore) GetID(publicKey []byte) ([]byte, error) {
	pubKey, err := crypto.NewPubKeyFromBytes(publicKey)
	if err != nil {
		return nil, fmt.Errorf("GetID error: %v", err)
	}

	programHash, err := program.CreateProgramHash(pubKey)
	if err != nil {
		return nil, fmt.Errorf("GetID error: %v", err)
	}

	return cs.States.GetID(programHash), nil
}

func (cs *ChainStore) GetNanoPay(addr Uint160, recipient Uint160, nonce uint64) (Fixed64, uint32, error) {
	return cs.States.GetNanoPay(addr, recipient, nonce)
}

type Donation struct {
	Height uint32
	Amount Fixed64
}

func NewDonation(height uint32, amount Fixed64) *Donation {
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

func (cs *ChainStore) GetDonation() (Fixed64, error) {
	donation, err := cs.getDonation()
	if err != nil {
		return Fixed64(0), err
	}
	return donation.Amount, nil
}

func (cs *ChainStore) getDonation() (*Donation, error) {
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

func (cs *ChainStore) CalcNextDonation(height uint32) (*Donation, error) {
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

	donationAddress, err := ToScriptHash(config.DonationAddress)
	if err != nil {
		return nil, err
	}
	account := cs.States.GetOrNewAccount(donationAddress)
	amount := account.GetBalance(config.NKNAssetID)
	donation := amount * config.DonationAdjustDividendFactor / config.DonationAdjustDivisorFactor
	donationPerBlock := int64(donation) / int64(config.RewardAdjustInterval)

	d := NewDonation(height, Fixed64(donationPerBlock))

	return d, nil
}

func (cs *ChainStore) GetStateRoots(fromHeight, toHeight uint32) ([]Uint256, error) {
	if toHeight < fromHeight {
		return nil, fmt.Errorf("toHeight(%v) is less than fromHeight(%v)\n", toHeight, fromHeight)
	}
	roots := make([]Uint256, 0, toHeight-fromHeight+1)

	for i := fromHeight; i <= toHeight; i++ {
		headerHash, err := cs.GetBlockHash(i)
		if err != nil {
			return nil, err
		}
		header, err := cs.GetHeader(headerHash)
		if err != nil {
			return nil, err
		}
		stateRoot, err := Uint256ParseFromBytes(header.UnsignedHeader.StateRoot)
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

func (cs *ChainStore) PruneStates() error {
	state, err := NewStateDB(EmptyUint256, cs)
	if err != nil {
		return err
	}

	return state.PruneStates()
}

func (cs *ChainStore) PruneStatesLowMemory() error {
	state, err := NewStateDB(EmptyUint256, cs)
	if err != nil {
		return err
	}

	return state.PruneStatesLowMemory()
}

func (cs *ChainStore) SequentialPrune() error {
	state, err := NewStateDB(EmptyUint256, cs)
	if err != nil {
		return err
	}

	return state.SequentialPrune()
}

func (cs *ChainStore) TrieTraverse() error {
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

	return states.TrieTraverse()
}
