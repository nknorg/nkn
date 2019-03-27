package chain

import (
	"math/rand"

	. "github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/crypto/util"
	. "github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	. "github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/vault"
	"github.com/nknorg/nkn/vm/signature"
	"github.com/nknorg/nnet/log"
)

type Mining interface {
	BuildBlock(height uint32, chordID []byte, winningHash common.Uint256, winnerType WinnerType, timestamp int64) (*Block, error)
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

func (bm *BuiltinMining) BuildBlock(height uint32, chordID []byte, winningHash common.Uint256, winnerType WinnerType, timestamp int64) (*Block, error) {
	var txnList []*Transaction
	var txnHashList []common.Uint256
	coinbase := bm.CreateCoinbaseTransaction()
	txnList = append(txnList, coinbase)
	txnHashList = append(txnHashList, coinbase.Hash())
	totalTxsSize := coinbase.GetSize()
	txCount := 1

	if winnerType == TXN_SIGNER {
		miningSigChainTxn, err := por.GetPorServer().GetMiningSigChainTxn(winningHash)
		if err != nil {
			return nil, err
		}
		txnList = append(txnList, miningSigChainTxn)
		txnHashList = append(txnHashList, miningSigChainTxn.Hash())
		totalTxsSize = totalTxsSize + miningSigChainTxn.GetSize()
		txCount++
	}

	txns, err := bm.txnCollector.Collect()
	if err != nil {
		return nil, err
	}

	for txnHash, txn := range txns {
		if txn.UnsignedTx.Fee < int64(config.Parameters.MinTxnFee) {
			log.Warning("transaction fee is too low")
			continue
		}

		totalTxsSize = totalTxsSize + txn.GetSize()
		if totalTxsSize > config.MaxBlockSize {
			break
		}

		txCount++
		if txCount > config.MaxNumTxnPerBlock {
			break
		}

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
	curStateHash := GenerateStateRoot(txnList)
	header := &Header{
		BlockHeader: BlockHeader{
			UnsignedHeader: &UnsignedHeader{
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
				ChordId:          chordID,
			},
			Signature: nil,
			Program: &Program{
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

	block := &Block{
		Header:       header,
		Transactions: txnList,
	}

	return block, nil
}

func (bm *BuiltinMining) CreateCoinbaseTransaction() *Transaction {
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

	payload := NewCoinbase(common.EmptyUint160, redeemHash, common.Fixed64(config.DefaultMiningReward*common.StorageFactor))
	pl, err := Pack(CoinbaseType, payload)
	if err != nil {
		return nil
	}

	txn := NewMsgTx(pl, rand.Uint64(), 0, util.RandomBytes(TransactionNonceLength))
	trans := &Transaction{
		MsgTx: *txn,
	}

	return trans
}
