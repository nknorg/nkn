package db

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/asset"
	"github.com/nknorg/nkn/core/contract/program"
	"github.com/nknorg/nkn/core/ledger"
	tx "github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/crypto/util"
)

func TestNewLedgerStore(t *testing.T) {
	ls, _ := NewLedgerStore()

	genesisBlock, _ := genesisBlockInit()
	ls.InitLedgerStoreWithGenesisBlock(genesisBlock, nil)

	b, err := BuildBlock(ls.GetCurrentBlockHash(), ls.GetHeight()+1)
	if err != nil {
		fmt.Println(err)
	}

	buff := bytes.NewBuffer(nil)
	b.Serialize(buff)
	fmt.Println("block", hex.EncodeToString(buff.Bytes()))

	if err := ls.SaveBlock(b); err != nil {
		fmt.Println(err)
	}
	ls.(*ChainStore).Dump()
	ls.Close()
}

func genesisBlockInit() (*ledger.Block, error) {
	genesisBlockHeader := &ledger.Header{
		Version:          1,
		PrevBlockHash:    common.Uint256{},
		TransactionsRoot: common.Uint256{},
		Timestamp:        time.Date(2018, time.January, 0, 0, 0, 0, 0, time.UTC).Unix(),
		Height:           uint32(0),
		ConsensusData:    100,
		NextBookKeeper:   common.Uint160{},
		Program: &program.Program{
			Code:      []byte{0x00},
			Parameter: []byte{0x00},
		},
	}
	// asset transaction
	trans := &tx.Transaction{
		TxType:         tx.RegisterAsset,
		PayloadVersion: 0,
		Payload: &payload.RegisterAsset{
			Asset: &asset.Asset{
				Name:        "NKN",
				Description: "NKN Test Token",
				Precision:   8,
			},
			Amount: 700000000 * 100000000,
			Issuer: &crypto.PubKey{
				X: big.NewInt(0),
				Y: big.NewInt(0),
			},
			Controller: common.Uint160{},
		},
		Attributes: []*tx.TxnAttribute{},
		Programs: []*program.Program{
			{
				Code:      []byte{0x00},
				Parameter: []byte{0x00},
			},
		},
	}
	// genesis block
	genesisBlock := &ledger.Block{
		Header:       genesisBlockHeader,
		Transactions: []*tx.Transaction{trans},
	}

	return genesisBlock, nil
}

func BuildBlock(parentHash common.Uint256, height uint32) (*ledger.Block, error) {
	var txnList []*tx.Transaction
	var txnHashList []common.Uint256
	coinbase := createCoinbaseTransaction()
	txnList = append(txnList, coinbase)
	txnHashList = append(txnHashList, coinbase.Hash())
	txnRoot, err := crypto.ComputeRoot(txnHashList)
	if err != nil {
		return nil, err
	}

	header := &ledger.Header{
		Version:          0,
		PrevBlockHash:    parentHash,
		Timestamp:        time.Now().Unix(),
		Height:           height,
		ConsensusData:    rand.Uint64(),
		TransactionsRoot: txnRoot,
		NextBookKeeper:   common.Uint160{},
		Signature:        nil,
		Program: &program.Program{
			Code:      []byte{0x00},
			Parameter: []byte{0x00},
		},
	}

	block := &ledger.Block{
		Header:       header,
		Transactions: txnList,
	}

	return block, nil
}
func createCoinbaseTransaction() *tx.Transaction {
	return &tx.Transaction{
		TxType:         tx.Coinbase,
		PayloadVersion: 0,
		Payload:        &payload.Coinbase{},
		Attributes: []*tx.TxnAttribute{
			{
				Usage: tx.Nonce,
				Data:  util.RandomBytes(tx.TransactionNonceLength),
			},
		},
		Inputs: []*tx.TxnInput{},
		Outputs: []*tx.TxnOutput{
			{
				AssetID:     common.Uint256{},
				Value:       10 * 100000000,
				ProgramHash: common.Uint160{},
			},
		},
		Programs: []*program.Program{},
	}
}
