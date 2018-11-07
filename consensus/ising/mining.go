package ising

import (
	"math/rand"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/core/contract/program"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/signature"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/crypto/util"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
)

type Mining interface {
	BuildBlock(height uint32, chordID []byte, winnerHash Uint256,
		winnerType ledger.WinnerType) (*ledger.Block, error)
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

func (bm *BuiltinMining) BuildBlock(height uint32, chordID []byte, winnerHash Uint256,
	winnerType ledger.WinnerType) (*ledger.Block, error) {
	var txnList []*transaction.Transaction
	var txnHashList []Uint256
	coinbase := bm.CreateCoinbaseTransaction()
	txnList = append(txnList, coinbase)
	txnHashList = append(txnHashList, coinbase.Hash())
	txns, err := bm.txnCollector.Collect(winnerHash)
	if err != nil {
		log.Error("collect transaction error: ", err)
		return nil, err
	}
	for txnHash, txn := range txns {
		if !ledger.DefaultLedger.Store.IsTxHashDuplicate(txnHash) {
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
	header := &ledger.Header{
		Version:          ledger.HeaderVersion,
		PrevBlockHash:    ledger.DefaultLedger.Store.GetCurrentBlockHash(),
		Timestamp:        time.Now().Unix(),
		Height:           height,
		ConsensusData:    rand.Uint64(),
		TransactionsRoot: txnRoot,
		NextBookKeeper:   Uint160{},
		WinnerHash:       winnerHash,
		WinnerType:	  winnerType,
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

	block := &ledger.Block{
		Header:       header,
		Transactions: txnList,
	}

	return block, nil
}

func (bm *BuiltinMining) CreateCoinbaseTransaction() *transaction.Transaction {
	// Transfer the reward to the beneficiary
	redeemHash := bm.account.ProgramHash
	if (config.Parameters.BeneficiaryAddr != "") {
		hash, err := ToScriptHash(config.Parameters.BeneficiaryAddr)
		if err == nil {
			redeemHash = hash
		} else {
			log.Error("%v\n%s", err, "Beneficiary account redeemhash convert failed")
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
				AssetID:     ledger.DefaultLedger.Blockchain.AssetID,
				Value:       Fixed64 (config.DefaultMiningReward * StorageFactor),
				ProgramHash: redeemHash,
			},
		},
		Programs: []*program.Program{},
	}
}
