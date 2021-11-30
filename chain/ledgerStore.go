package chain

import (
	"context"

	"github.com/nknorg/nkn/v2/chain/db"

	"github.com/nknorg/nkn/v2/block"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/transaction"
)

// ILedgerStore provides func with store package.
type ILedgerStore interface {
	SaveBlock(b *block.Block, pruning, fastSync bool) error
	GetBlock(hash common.Uint256) (*block.Block, error)
	GetBlockByHeight(height uint32) (*block.Block, error)
	GetBlockHash(height uint32) (common.Uint256, error)
	IsDoubleSpend(tx *transaction.Transaction) bool
	AddHeader(header *block.Header) error
	GetHeader(hash common.Uint256) (*block.Header, error)
	GetHeaderByHeight(height uint32) (*block.Header, error)
	GetTransaction(hash common.Uint256) (*transaction.Transaction, error)
	GetName_legacy(registrant []byte) (string, error)
	GetRegistrant(name string) ([]byte, uint32, error)
	GetRegistrant_legacy(name string) ([]byte, error)
	IsSubscribed(topic string, bucket uint32, subscriber []byte, identifier string) (bool, error)
	GetSubscription(topic string, bucket uint32, subscriber []byte, identifier string) (string, uint32, error)
	GetSubscribers(topic string, bucket, offset, limit uint32, subscriberHashPrefix []byte, ctx context.Context) ([]string, error)
	GetSubscribersWithMeta(topic string, bucket, offset, limit uint32, subscriberHashPrefix []byte, ctx context.Context) (map[string]string, error)
	GetSubscribersCount(topic string, bucket uint32, subscriberHashPrefix []byte, ctx context.Context) (int, error)
	GetID(publicKey []byte, height uint32) ([]byte, error)
	GetIDVersion(publicKey []byte) ([]byte, byte, error)
	GetBalance(addr common.Uint160) common.Fixed64
	GetBalanceByAssetID(addr common.Uint160, assetID common.Uint256) common.Fixed64
	GetNonce(addr common.Uint160) uint64
	GetNanoPay(addr common.Uint160, recipient common.Uint160, nonce uint64) (common.Fixed64, uint32, error)
	GetCurrentBlockHash() common.Uint256
	GetCurrentHeaderHash() common.Uint256
	GetHeaderHeight() uint32
	GetHeight() uint32
	GetHeightByBlockHash(hash common.Uint256) (uint32, error)
	GetHeaderHashByHeight(height uint32) common.Uint256
	GetHeaderWithCache(hash common.Uint256) (*block.Header, error)
	GetSigChainWithCache(hash common.Uint256) (*pb.SigChain, error)
	InitLedgerStoreWithGenesisBlock(genesisblock *block.Block) (uint32, error)
	GetDonation() (common.Fixed64, error)
	IsTxHashDuplicate(txhash common.Uint256) bool
	IsBlockInStore(hash common.Uint256) bool
	Rollback(b *block.Block) error
	GenerateStateRoot(ctx context.Context, b *block.Block, genesisBlockInitialized, needBeCommitted bool) (common.Uint256, error)
	GetAsset(assetID common.Uint256) (name, symbol string, totalSupply common.Fixed64, precision uint32, err error)
	GetDatabase() db.IStore
	ShouldFastSync(syncStopHeight uint32) bool
	GetSyncRootHeight() (uint32, error)
	GetFastSyncStateRoot() (common.Uint256, error)
	PrepareFastSync(fastSyncHeight uint32, fastSyncRootHash common.Uint256) error
	FastSyncDone(syncRootHash common.Uint256, fastSyncHeight uint32) error
	Close()
}
