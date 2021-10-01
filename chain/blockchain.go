package chain

import (
	"context"
	"sync"

	"github.com/nknorg/nkn/v2/block"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/event"
	"github.com/nknorg/nkn/v2/util/log"
)

type Blockchain struct {
	BlockHeight      uint32
	AssetID          common.Uint256
	BlockPersistTime map[common.Uint256]int64
	mutex            sync.Mutex
	muTime           sync.Mutex
}

func NewBlockchain(height uint32, asset common.Uint256) *Blockchain {
	return &Blockchain{
		BlockHeight:      height,
		AssetID:          asset,
		BlockPersistTime: make(map[common.Uint256]int64),
	}
}

func NewBlockchainWithGenesisBlock(store ILedgerStore) (*Blockchain, error) {
	genesisBlock, err := block.GenesisBlockInit()
	if err != nil {
		return nil, err
	}

	root, err := store.GenerateStateRoot(context.Background(), genesisBlock, false, false)
	if err != nil {
		return nil, err
	}

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

func (bc *Blockchain) AddBlock(block *block.Block, pruning, fastSync bool) error {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	err := bc.SaveBlock(block, pruning, fastSync)
	if err != nil {
		log.Errorf("AddBlock error: %v", err)
		return err
	}

	return nil
}

func (bc *Blockchain) SaveBlock(block *block.Block, pruning, fastSync bool) error {
	err := HeaderCheck(block.Header, fastSync)
	if err != nil {
		return err
	}

	err = TransactionCheck(context.Background(), block, fastSync)
	if err != nil {
		return err
	}

	err = DefaultLedger.Store.SaveBlock(block, pruning, fastSync)
	if err != nil {
		return err
	}

	log.Infof("# current block height: %d, block hash: %x", block.Header.UnsignedHeader.Height, block.Hash())

	bc.BlockHeight = block.Header.UnsignedHeader.Height

	event.Queue.Notify(event.BlockPersistCompleted, block)

	return nil
}

func (bc *Blockchain) GetHeader(hash common.Uint256) (*block.Header, error) {
	header, err := DefaultLedger.Store.GetHeader(hash)
	if err != nil {
		return nil, err
	}
	return header, nil
}

func (bc *Blockchain) AddHeader(header *block.Header, fastSync bool) error {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	err := HeaderCheck(header, fastSync)
	if err != nil {
		return err
	}

	err = DefaultLedger.Store.AddHeader(header)
	if err != nil {
		return err
	}

	log.Infof("# current header height: %d, header hash: %x", header.UnsignedHeader.Height, header.Hash())

	return nil
}
