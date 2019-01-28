package core

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/types"
)

// ILedgerStore provides func with store package.
type ILedgerStore interface {
	SaveBlock(b *types.Block, ledger *Ledger, fastAdd bool) error
	GetBlock(hash Uint256) (*types.Block, error)
	GetBlockByHeight(height uint32) (*types.Block, error)
	//BlockInCache(hash Uint256) bool
	GetBlockHash(height uint32) (Uint256, error)
	//GetBlockHistory(startHeight, blockNum uint32) map[uint32]Uint256
	//CheckBlockHistory(history map[uint32]Uint256) (uint32, bool)
	//GetVotingWeight(hash Uint160) (int, error)

	IsDoubleSpend(tx *types.Transaction) bool

	AddHeaders(headers []*types.Header) error
	GetHeader(hash Uint256) (*types.Header, error)
	GetHeaderByHeight(height uint32) (*types.Header, error)

	GetTransaction(hash Uint256) (*types.Transaction, error)

	//SaveAsset(assetid Uint256, asset *Asset) error
	//GetAsset(hash Uint256) (*Asset, error)

	SaveName(registrant []byte, name string) error
	GetName(registrant []byte) (*string, error)
	GetRegistrant(name string) ([]byte, error)

	IsSubscribed(subscriber []byte, identifier string, topic string, bucket uint32) (bool, error)
	GetSubscribers(topic string, bucket uint32) map[string]string
	GetSubscribersCount(topic string, bucket uint32) int
	GetFirstAvailableTopicBucket(topic string) int
	GetTopicBucketsCount(topic string) uint32

	GetStateRootHash() Uint256
	GetBalance(addr Uint160) Fixed64

	//GetContract(codeHash Uint160) ([]byte, error)
	//GetStorage(key []byte) ([]byte, error)

	GetCurrentBlockHash() Uint256
	GetCurrentHeaderHash() Uint256
	GetHeaderHeight() uint32
	GetHeight() uint32
	GetHeightByBlockHash(hash Uint256) (uint32, error)
	GetHeaderHashByHeight(height uint32) Uint256

	InitLedgerStoreWithGenesisBlock(genesisblock *types.Block) (uint32, error)

	//GetQuantityIssued(assetid Uint256) (Fixed64, error)
	//GetUnspent(txid Uint256, index uint16) (*tx.TxnOutput, error)
	//ContainsUnspent(txid Uint256, index uint16) (bool, error)
	//GetUnspentFromProgramHash(programHash Uint160, assetid Uint256) ([]*tx.UTXOUnspent, error)
	//GetUnspentsFromProgramHash(programHash Uint160) (map[Uint256][]*tx.UTXOUnspent, error)
	//GetPrepaidInfo(programHash Uint160) (*Fixed64, *Fixed64, error)

	//GetAssets() map[Uint256]*Asset

	IsTxHashDuplicate(txhash Uint256) bool
	IsBlockInStore(hash Uint256) bool
	Rollback(b *types.Block) error
	Close()
}
