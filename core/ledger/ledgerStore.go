package ledger

import (
	. "nkn-core/common"
	. "nkn-core/core/asset"
)

// ILedgerStore provides func with store package.
type ILedgerStore interface {
	//TODO: define the state store func
	SaveBlock(b *Block, ledger *Ledger) error
	GetBlock(hash Uint256) (*Block, error)
	BlockInCache(hash Uint256) bool
	GetBlockHash(height uint32) (Uint256, error)
	InitLedgerStore(ledger *Ledger) error
	IsDoubleSpend(tx *Transaction) bool

	//SaveHeader(header *Header,ledger *Ledger) error
	AddHeaders(headers []BlockHeader, ledger *Ledger) error
	GetHeader(hash Uint256) (*BlockHeader, error)

	GetTransaction(hash Uint256) (*Transaction, error)

	SaveAsset(assetid Uint256, asset *Asset) error
	GetAsset(hash Uint256) (*Asset, error)

	GetContract(codeHash Uint160) ([]byte, error)
	GetStorage(key []byte) ([]byte, error)

	GetCurrentBlockHash() Uint256
	GetCurrentHeaderHash() Uint256
	GetHeaderHeight() uint32
	GetHeight() uint32
	GetHeaderHashByHeight(height uint32) Uint256

	InitLedgerStoreWithGenesisBlock(genesisblock *Block) (uint32, error)

	GetUnspent(txid Uint256, index uint16) (*TxOutput, error)
	ContainsUnspent(txid Uint256, index uint16) (bool, error)
	GetUnspentFromProgramHash(programHash Uint160, assetid Uint256) ([]*UTXOUnspent, error)
	GetUnspentsFromProgramHash(programHash Uint160) (map[Uint256][]*UTXOUnspent, error)
	GetAssets() map[Uint256]*Asset

	IsTxHashDuplicate(txhash Uint256) bool
	IsBlockInStore(hash Uint256) bool
	Close()
}
