package ledger

import (
	. "github.com/nknorg/nkn/common"
	. "github.com/nknorg/nkn/core/asset"
	tx "github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/crypto"
)

// ILedgerStore provides func with store package.
type ILedgerStore interface {
	SaveBlock(b *Block, ledger *Ledger) error
	GetBlock(hash Uint256) (*Block, error)
	BlockInCache(hash Uint256) bool
	GetBlockHash(height uint32) (Uint256, error)
	GetBlockHistory(startHeight, blockNum uint32) map[uint32]Uint256
	CheckBlockHistory(history map[uint32]Uint256) (uint32, bool)
	GetVotingWeight(hash Uint160) (int, error)

	IsDoubleSpend(tx *tx.Transaction) bool

	AddHeaders(headers []Header) error
	GetHeader(hash Uint256) (*Header, error)

	GetTransaction(hash Uint256) (*tx.Transaction, error)

	GetAsset(hash Uint256) (*Asset, error)

	GetCurrentBlockHash() Uint256
	GetCurrentCachedHeaderHash() Uint256
	GetCurrentCachedHeaderHeight() uint32
	GetHeight() uint32
	GetHeightByBlockHash(hash Uint256) (uint32, error)
	GetCachedHeaderHash(height uint32) Uint256

	GetBookKeeperList() ([]*crypto.PubKey, []*crypto.PubKey, error)
	InitLedgerStoreWithGenesisBlock(genesisblock *Block, defaultBookKeeper []*crypto.PubKey) (uint32, error)

	GetQuantityIssued(assetid Uint256) (Fixed64, error)

	GetUnspentFromProgramHash(programHash Uint160, assetid Uint256) ([]*tx.UTXOUnspent, error)
	GetUnspentsFromProgramHash(programHash Uint160) (map[Uint256][]*tx.UTXOUnspent, error)
	GetPrepaidInfo(programHash Uint160) (*Fixed64, *Fixed64, error)

	GetAssets() map[Uint256]*Asset

	IsTxHashDuplicate(txhash Uint256) bool
	IsBlockInStore(hash Uint256) bool
	Close()
}
