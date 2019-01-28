package core

import (
	"math/rand"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/crypto/util"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/signature"
	"github.com/nknorg/nkn/types"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/vault"
	"github.com/nknorg/nnet/log"
)

type Mining interface {
	BuildBlock(height uint32, chordID []byte, winningHash common.Uint256, winnerType types.WinnerType, timestamp int64) (*types.Block, error)
}

type BuiltinMining struct {
	account      *vault.Account // local account
	txnCollector *TxnCollector  // transaction pool
}

func NewBuiltinMining(account *vault.Account, txnCollector *TxnCollector) *BuiltinMining {
	return &BuiltinMining{
		account:      account,
		txnCollector: txnCollector,
	}
}

func (bm *BuiltinMining) BuildBlock(height uint32, chordID []byte, winningHash common.Uint256, winnerType types.WinnerType, timestamp int64) (*types.Block, error) {
	var txnList []*types.Transaction
	var txnHashList []common.Uint256
	coinbase := bm.CreateCoinbaseTransaction()
	txnList = append(txnList, coinbase)
	txnHashList = append(txnHashList, coinbase.Hash())

	if winnerType == types.TxnSigner {
		miningSigChainTxn, err := por.GetPorServer().GetMiningSigChainTxn(winningHash)
		if err != nil {
			return nil, err
		}
		txnList = append(txnList, miningSigChainTxn)
		txnHashList = append(txnHashList, miningSigChainTxn.Hash())
	}

	txns, err := bm.txnCollector.Collect()
	if err != nil {
		return nil, err
	}
	for txnHash, txn := range txns {
		if !DefaultLedger.Store.IsTxHashDuplicate(txnHash) {
			txnList = append(txnList, txn)
			txnHashList = append(txnHashList, txnHash)
		}
	}
	txnRoot, err := crypto.ComputeRoot(txnHashList)
	if err != nil {
		return nil, err
	}
	encodedPubKey, err := bm.account.PublicKey.EncodePoint(true)
	if err != nil {
		return nil, err
	}
	curBlockHash := DefaultLedger.Store.GetCurrentBlockHash()
	curStateHash := DefaultLedger.Store.GetStateRootHash()
	header := &types.Header{
		BlockHeader: types.BlockHeader{
			UnsignedHeader: &types.UnsignedHeader{
				Version:          HeaderVersion,
				PrevBlockHash:    curBlockHash.ToArray(),
				Timestamp:        timestamp,
				Height:           height,
				ConsensusData:    rand.Uint64(),
				TransactionsRoot: txnRoot.ToArray(),
				StateRoot:        curStateHash.ToArray(),
				NextBookKeeper:   common.EmptyUint160.ToArray(),
				WinnerHash:       winningHash.ToArray(),
				WinnerType:       winnerType,
				Signer:           encodedPubKey,
				ChordID:          chordID,
			},
			Signature: nil,
			Program: &types.Program{
				Code:      []byte{0x00},
				Parameter: []byte{0x00},
			},
		},
	}

	hash := signature.GetHashForSigning(header)
	sig, err := crypto.Sign(bm.account.PrivateKey, hash)
	if err != nil {
		return nil, err
	}
	header.Signature = append(header.Signature, sig...)

	block := &types.Block{
		Header:       header,
		Transactions: txnList,
	}

	return block, nil
}

func (bm *BuiltinMining) CreateCoinbaseTransaction() *types.Transaction {
	// Transfer the reward to the beneficiary
	redeemHash := bm.account.ProgramHash
	if config.Parameters.BeneficiaryAddr != "" {
		hash, err := common.ToScriptHash(config.Parameters.BeneficiaryAddr)
		if err == nil {
			redeemHash = hash
		} else {
			log.Errorf("Convert beneficiary account to redeemhash error: %v", err)
		}
	}

	payload := types.NewCoinbase(common.EmptyUint160, redeemHash, common.Fixed64(config.DefaultMiningReward*common.StorageFactor))
	pl, err := types.Pack(types.CoinbaseType, payload)
	if err != nil {
		return nil
	}

	txn := types.NewMsgTx(pl, rand.Uint64(), 0, util.RandomBytes(types.TransactionNonceLength))
	trans := &types.Transaction{
		MsgTx: *txn,
	}

	return trans
}
