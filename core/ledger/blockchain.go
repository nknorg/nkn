package ledger

import (
	"errors"
	"sync"

	. "github.com/nknorg/nkn/common"
	tx "github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/util/log"
)

type Blockchain struct {
	BlockHeight      uint32
	AssetID          Uint256
	BlockPersistTime map[Uint256]int64
	BCEvents         *events.Event
	mutex            sync.Mutex
}

func NewBlockchain(height uint32, asset Uint256) *Blockchain {
	return &Blockchain{
		BlockHeight:      height,
		AssetID:          asset,
		BlockPersistTime: make(map[Uint256]int64),
		BCEvents:         events.NewEvent(),
	}
}

func NewBlockchainWithGenesisBlock(store ILedgerStore, defaultBookKeeper []*crypto.PubKey) (*Blockchain, error) {
	genesisBlock, err := GenesisBlockInit()
	if err != nil {
		return nil, err
	}
	genesisBlock.RebuildMerkleRoot()
	hashx := genesisBlock.Hash()
	genesisBlock.hash = &hashx

	height, err := store.InitLedgerStoreWithGenesisBlock(genesisBlock, defaultBookKeeper)
	if err != nil {
		return nil, err
	}
	blockchain := NewBlockchain(height, genesisBlock.Transactions[0].Hash())

	return blockchain, nil
}

func (bc *Blockchain) AddBlock(block *Block) error {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	err := bc.SaveBlock(block)
	if err != nil {
		return err
	}

	return nil
}

func (bc *Blockchain) GetHeader(hash Uint256) (*Header, error) {
	header, err := DefaultLedger.Store.GetHeader(hash)
	if err != nil {
		return nil, err
	}
	return header, nil
}

func (bc *Blockchain) SaveBlock(block *Block) error {
	err := DefaultLedger.Store.SaveBlock(block, DefaultLedger)
	if err != nil {
		log.Warn("Save Block failure , ", err)
		return err
	}

	return nil
}

func (bc *Blockchain) ContainsTransaction(hash Uint256) bool {
	//TODO: implement error catch
	_, err := DefaultLedger.Store.GetTransaction(hash)
	if err != nil {
		return false
	}
	return true
}

func (bc *Blockchain) GetBookKeepersByTXs(others []*tx.Transaction) []*crypto.PubKey {
	//TODO: GetBookKeepers()
	//TODO: Just for TestUse

	return StandbyBookKeepers
}

func (bc *Blockchain) GetBookKeepers() []*crypto.PubKey {
	//TODO: GetBookKeepers()
	//TODO: Just for TestUse

	return StandbyBookKeepers
}

func (bc *Blockchain) CurrentBlockHash() Uint256 {
	return DefaultLedger.Store.GetCurrentBlockHash()
}

func (bc *Blockchain) AddBlockTime(hash Uint256, time int64) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	bc.BlockPersistTime[hash] = time
}

func (bc *Blockchain) GetBlockTime(hash Uint256) (int64, error) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	// use previous timestamp as block persisted time when cache is empty
	if len(bc.BlockPersistTime) == 0 {
		block, err := DefaultLedger.Store.GetBlock(hash)
		if err != nil {
			return 0, err
		}
		return block.Header.Timestamp, nil
	}

	if time, ok := bc.BlockPersistTime[hash]; ok {
		return time, nil
	}

	return 0, errors.New("no receive time for block")
}
