package ledger

import (
	. "nkn-core/common"
	. "nkn-core/core/asset"
)

type ILedgerStore interface {
	InitLedgerStoreWithGenesisBlock(genesisblock *Block) (uint32, error)

	SaveBlock(b *Block) error
	RollbackBlock(blockHash Uint256) error
	AddHeaders(headers []BlockHeader, ledger *Ledger) error

	GetBlock(hash Uint256) (*Block, error)
	GetHeader(hash Uint256) (*BlockHeader, error)
	GetBlockHeight() uint32
	GetHeaderHeight() uint32
	GetBlockHashByHeight(height uint32) (Uint256, error)
	GetHeaderHashByHeight(height uint32) Uint256
	GetCurrentBlockHash() Uint256
	GetCurrentHeaderHash() Uint256
	GetAsset(hash Uint256) (*Asset, error)
	GetUnspentFromProgramHash(programHash Uint160, assetid Uint256) ([]*UTXOUnspent, error)
	GetUnspentsFromProgramHash(programHash Uint160) (map[Uint256][]*UTXOUnspent, error)
	GetTransaction(hash Uint256) (*Transaction, error)
	GetUnspent(txid Uint256, index uint16) (*TxOutput, error)
	GetHeaderHashFront() (Uint256, error)
	GetHeaderHashNext(prevHash Uint256) (Uint256, error)
	RemoveHeaderListElement(hash Uint256)

	ContainsUnspent(txid Uint256, index uint16) (bool, error)
	IsTxHashDuplicate(txhash Uint256) bool
	IsDoubleSpend(tx *Transaction) bool

	Close()
}
