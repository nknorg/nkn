package chain

import (
	"github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/chain/db"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/transaction"
)

// ILedgerStore provides func with store package.
type ILedgerStore interface {
	SaveBlock(b *block.Block, fastAdd bool) error
	GetBlock(hash Uint256) (*block.Block, error)
	GetBlockByHeight(height uint32) (*block.Block, error)
	GetBlockHash(height uint32) (Uint256, error)
	IsDoubleSpend(tx *transaction.Transaction) bool
	AddHeader(header *block.Header) error
	GetHeader(hash Uint256) (*block.Header, error)
	GetHeaderByHeight(height uint32) (*block.Header, error)
	GetTransaction(hash Uint256) (*transaction.Transaction, error)
	SaveName(registrant []byte, name string) error
	GetName(registrant []byte) (*string, error)
	GetRegistrant(name string) ([]byte, error)
	IsSubscribed(subscriber []byte, identifier string, topic string, bucket uint32) (bool, error)
	GetSubscribers(topic string, bucket uint32) map[string]string
	GetSubscribersCount(topic string, bucket uint32) int
	GetFirstAvailableTopicBucket(topic string) int
	GetTopicBucketsCount(topic string) uint32
	GetID(publicKey []byte) ([]byte, error)
	GetDatabase() db.IStore
	GetCurrentBlockStateRoot() Uint256
	GetStateRootHash() Uint256
	GetBalance(addr Uint160) Fixed64
	GetNonce(addr Uint160) uint64
	GetCurrentBlockHash() Uint256
	GetCurrentHeaderHash() Uint256
	GetHeaderHeight() uint32
	GetHeight() uint32
	GetHeightByBlockHash(hash Uint256) (uint32, error)
	GetHeaderHashByHeight(height uint32) Uint256
	GetHeaderWithCache(hash Uint256) (*block.Header, error)
	InitLedgerStoreWithGenesisBlock(genesisblock *block.Block) (uint32, error)
	GetDonation() (*db.Donation, error)
	IsTxHashDuplicate(txhash Uint256) bool
	IsBlockInStore(hash Uint256) bool
	Rollback(b *block.Block) error
	Close()
}
