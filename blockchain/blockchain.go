package blockchain

import (
	"errors"
	"sort"
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/types"
	"github.com/nknorg/nkn/util/log"
)

type Blockchain struct {
	BlockHeight      uint32
	AssetID          Uint256
	BlockPersistTime map[Uint256]int64
	BCEvents         *events.Event
	mutex            sync.Mutex
	muTime           sync.Mutex
}

func NewBlockchain(height uint32, asset Uint256) *Blockchain {
	return &Blockchain{
		BlockHeight:      height,
		AssetID:          asset,
		BlockPersistTime: make(map[Uint256]int64),
		BCEvents:         events.NewEvent(),
	}
}

func NewBlockchainWithGenesisBlock(store ILedgerStore) (*Blockchain, error) {
	genesisBlock, err := types.GenesisBlockInit()
	if err != nil {
		return nil, err
	}
	root := GenesisStateRoot(store, genesisBlock.Transactions)
	genesisBlock.Header.UnsignedHeader.StateRoot = root.ToArray()
	genesisBlock.RebuildMerkleRoot()
	genesisBlock.Hash()

	height, err := store.InitLedgerStoreWithGenesisBlock(genesisBlock)
	if err != nil {
		return nil, err
	}

	blockchain := NewBlockchain(height, genesisBlock.Transactions[0].Hash())

	return blockchain, nil
}

func (bc *Blockchain) AddBlock(block *types.Block, fastAdd bool) error {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	err := bc.SaveBlock(block, fastAdd)
	if err != nil {
		return err
	}

	return nil
}

func (bc *Blockchain) GetHeader(hash Uint256) (*types.Header, error) {
	header, err := DefaultLedger.Store.GetHeader(hash)
	if err != nil {
		return nil, err
	}
	return header, nil
}

func (bc *Blockchain) SaveBlock(block *types.Block, fastAdd bool) error {
	err := DefaultLedger.Store.SaveBlock(block, fastAdd)
	if err != nil {
		log.Warning("Save Block failure , ", err)
		return err
	}

	bc.BlockHeight = block.Header.UnsignedHeader.Height
	bc.BCEvents.Notify(events.EventBlockPersistCompleted, block)
	log.Infof("# current block height: %d, block hash: %x", bc.BlockHeight, block.Hash())

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

func (bc *Blockchain) CurrentBlockHash() Uint256 {
	return DefaultLedger.Store.GetCurrentBlockHash()
}

func (bc *Blockchain) AddHeaders(headers []*types.Header) error {
	//TODO mutex
	sort.Slice(headers, func(i, j int) bool {
		return headers[i].UnsignedHeader.Height < headers[j].UnsignedHeader.Height
	})

	for i := 0; i < len(headers); i++ {
		if !VerifyHeader(headers[i]) {
			return errors.New("header verify error.")
		}

		DefaultLedger.Store.AddHeader(headers[i])
	}

	return nil
}
