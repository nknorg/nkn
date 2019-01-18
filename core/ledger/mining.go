package ledger

import (
	"math/rand"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/contract/program"
	"github.com/nknorg/nkn/core/signature"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/crypto/util"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/vault"
	"github.com/nknorg/nnet/log"
)

type Mining interface {
	BuildBlock(height uint32, chordID []byte, winningHash common.Uint256, winnerType WinnerType, timestamp int64) (*Block, error)
}

type BuiltinMining struct {
	account      *vault.Account            // local account
	txnCollector *transaction.TxnCollector // transaction pool
}

func NewBuiltinMining(account *vault.Account, txnCollector *transaction.TxnCollector) *BuiltinMining {
	return &BuiltinMining{
		account:      account,
		txnCollector: txnCollector,
	}
}

func (bm *BuiltinMining) BuildBlock(height uint32, chordID []byte, winningHash common.Uint256, winnerType WinnerType, timestamp int64) (*Block, error) {
	var txnList []*transaction.Transaction
	var txnHashList []common.Uint256
	coinbase := bm.CreateCoinbaseTransaction()
	txnList = append(txnList, coinbase)
	txnHashList = append(txnHashList, coinbase.Hash())

	if winnerType == TxnSigner {
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
	header := &Header{
		Version:          HeaderVersion,
		PrevBlockHash:    DefaultLedger.Store.GetCurrentBlockHash(),
		Timestamp:        timestamp,
		Height:           height,
		ConsensusData:    rand.Uint64(),
		TransactionsRoot: txnRoot,
		NextBookKeeper:   common.Uint160{},
		WinnerHash:       winningHash,
		WinnerType:       winnerType,
		Signer:           encodedPubKey,
		ChordID:          chordID,
		Signature:        nil,
		Program: &program.Program{
			Code:      []byte{0x00},
			Parameter: []byte{0x00},
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

func (bm *BuiltinMining) CreateCoinbaseTransaction() *transaction.Transaction {
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

	return &transaction.Transaction{
		TxType:         transaction.Coinbase,
		PayloadVersion: 0,
		Payload:        &payload.Coinbase{},
		Attributes: []*transaction.TxnAttribute{
			{
				Usage: transaction.Nonce,
				Data:  util.RandomBytes(transaction.TransactionNonceLength),
			},
		},
		Inputs: []*transaction.TxnInput{},
		Outputs: []*transaction.TxnOutput{
			{
				AssetID:     DefaultLedger.Blockchain.AssetID,
				Value:       common.Fixed64(config.DefaultMiningReward * common.StorageFactor),
				ProgramHash: redeemHash,
			},
		},
		Programs: []*program.Program{},
	}
}
