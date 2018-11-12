package ledger

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/account"
	. "github.com/nknorg/nkn/core/asset"
	tx "github.com/nknorg/nkn/core/transaction"
)

// ILedgerStore provides func with store package.
type ILedgerStore interface {
	SaveBlock(b *Block, ledger *Ledger) error
	GetBlock(hash Uint256) (*Block, error)
	GetBlockByHeight(height uint32) (*Block, error)
	BlockInCache(hash Uint256) bool
	GetBlockHash(height uint32) (Uint256, error)
	GetBlockHistory(startHeight, blockNum uint32) map[uint32]Uint256
	CheckBlockHistory(history map[uint32]Uint256) (uint32, bool)
	GetVotingWeight(hash Uint160) (int, error)

	IsDoubleSpend(tx *tx.Transaction) bool

	AddHeaders(headers []Header, ledger *Ledger) error
	GetHeader(hash Uint256) (*Header, error)

	GetTransaction(hash Uint256) (*tx.Transaction, error)

	SaveAsset(assetid Uint256, asset *Asset) error
	GetAsset(hash Uint256) (*Asset, error)

	SaveName(registrant []byte, name string) error
	GetName(registrant []byte) (*string, error)
	GetRegistrant(name string) ([]byte, error)

	GetContract(codeHash Uint160) ([]byte, error)
	GetStorage(key []byte) ([]byte, error)
	GetAccount(programHash Uint160) (*account.AccountState, error)

	GetCurrentBlockHash() Uint256
	GetCurrentHeaderHash() Uint256
	GetHeaderHeight() uint32
	GetHeight() uint32
	GetHeightByBlockHash(hash Uint256) (uint32, error)
	GetHeaderHashByHeight(height uint32) Uint256

	InitLedgerStoreWithGenesisBlock(genesisblock *Block) (uint32, error)

	GetQuantityIssued(assetid Uint256) (Fixed64, error)

	GetUnspent(txid Uint256, index uint16) (*tx.TxnOutput, error)
	ContainsUnspent(txid Uint256, index uint16) (bool, error)
	GetUnspentFromProgramHash(programHash Uint160, assetid Uint256) ([]*tx.UTXOUnspent, error)
	GetUnspentsFromProgramHash(programHash Uint160) (map[Uint256][]*tx.UTXOUnspent, error)
	GetPrepaidInfo(programHash Uint160) (*Fixed64, *Fixed64, error)

	GetAssets() map[Uint256]*Asset

	IsTxHashDuplicate(txhash Uint256) bool
	IsBlockInStore(hash Uint256) bool
	Rollback(b *Block) error
	Close()
}
