package chain

import (
	"context"

	"github.com/nknorg/nkn/block"
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
	GetName(registrant []byte) (string, error)
	GetRegistrant(name string) ([]byte, error)
	IsSubscribed(topic string, bucket uint32, subscriber []byte, identifier string) (bool, error)
	GetSubscription(topic string, bucket uint32, subscriber []byte, identifier string) (string, uint32, error)
	GetSubscribers(topic string, bucket, offset, limit uint32) ([]string, error)
	GetSubscribersWithMeta(topic string, bucket, offset, limit uint32) (map[string]string, error)
	GetSubscribersCount(topic string, bucket uint32) int
	GetID(publicKey []byte) ([]byte, error)
	GetBalance(addr Uint160) Fixed64
	GetBalanceByAssetID(addr Uint160, assetID Uint256) Fixed64
	GetNonce(addr Uint160) uint64
	GetNanoPay(addr Uint160, recipient Uint160, nonce uint64) (Fixed64, uint32, error)
	GetCurrentBlockHash() Uint256
	GetCurrentHeaderHash() Uint256
	GetHeaderHeight() uint32
	GetHeight() uint32
	GetHeightByBlockHash(hash Uint256) (uint32, error)
	GetHeaderHashByHeight(height uint32) Uint256
	GetHeaderWithCache(hash Uint256) (*block.Header, error)
	InitLedgerStoreWithGenesisBlock(genesisblock *block.Block) (uint32, error)
	GetDonation() (Fixed64, error)
	IsTxHashDuplicate(txhash Uint256) bool
	IsBlockInStore(hash Uint256) bool
	Rollback(b *block.Block) error
	GenerateStateRoot(ctx context.Context, b *block.Block, genesisBlockInitialized, needBeCommitted bool) (Uint256, error)
	GetAsset(assetID Uint256) (name, symbol string, totalSupply Fixed64, precision uint32, err error)

	Close()
}
