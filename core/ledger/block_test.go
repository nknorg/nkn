package ledger

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/contract/program"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
)

func TestHeader(t *testing.T) {
	header := &Header{
		Version:          12,
		PrevBlockHash:    common.Uint256{100, 102},
		TransactionsRoot: common.Uint256{2, 3, 6, 7},
		Timestamp:        8848,
		Height:           3721,
		ConsensusData:    5566,
		NextBookKeeper:   common.Uint160{1, 6, 0},
		Program: &program.Program{
			Code:      []byte{1, 2, 3, 4},
			Parameter: []byte{5, 6, 7, 8},
		},
		hash: common.Uint256{2, 5, 6},
	}

	data, err := header.MarshalJson()
	if err != nil {
		t.Error("Header MarshalJson error")
	}

	var x interface{}
	json.Unmarshal(data, &x)

	newHeader := new(Header)
	err = newHeader.UnmarshalJson(data)
	if err != nil {
		t.Error("Header UnmarshalJson error")
	}
}

func TestBlock(t *testing.T) {
	block := &Block{
		//hash: &common.Uint256{3, 3, 7, 7},
		Header: &Header{
			Version:          12,
			PrevBlockHash:    common.Uint256{100, 102},
			TransactionsRoot: common.Uint256{2, 3, 6, 7},
			Timestamp:        8848,
			Height:           3721,
			ConsensusData:    5566,
			NextBookKeeper:   common.Uint160{1, 6, 0},
			Program: &program.Program{
				Code:      []byte{1, 2, 3, 4},
				Parameter: []byte{5, 6, 7, 8},
			},
			hash: common.Uint256{2, 5, 6},
		},
		Transactions: []*transaction.Transaction{
			&transaction.Transaction{
				TxType:         0x10,
				PayloadVersion: 1,
				Payload: &payload.Commit{
					SigChain:  []byte{1, 2, 3, 4},
					Submitter: common.Uint160{5, 6, 7, 8},
				},
				Attributes: []*transaction.TxnAttribute{
					&transaction.TxnAttribute{
						Usage: 1,
						Data:  []byte{1, 2, 3, 4},
						Size:  5,
					},
					&transaction.TxnAttribute{
						Usage: 1,
						Data:  []byte{3, 6, 2, 1},
					},
				},
				Inputs: []*transaction.TxnInput{
					&transaction.TxnInput{
						ReferTxID:          common.Uint256{0, 1, 2, 4},
						ReferTxOutputIndex: 5,
					},
					&transaction.TxnInput{
						ReferTxID:          common.Uint256{6, 7},
						ReferTxOutputIndex: 3,
					},
				},
				Outputs: []*transaction.TxnOutput{
					&transaction.TxnOutput{
						AssetID:     common.Uint256{0, 1, 2, 3, 4},
						Value:       common.Fixed64(314159260),
						ProgramHash: common.Uint160{1, 2, 3, 4, 5, 6, 7, 8},
					},
					&transaction.TxnOutput{
						AssetID:     common.Uint256{0, 1, 3, 4},
						Value:       common.Fixed64(7788),
						ProgramHash: common.Uint160{1, 2, 5, 6, 7, 8},
					},
				},
				Programs: []*program.Program{
					&program.Program{
						Code:      []byte{1, 2, 3, 4},
						Parameter: []byte{5, 6, 7, 8},
					},
					&program.Program{
						Code:      []byte{1, 23, 4},
						Parameter: []byte{5, 6, 78},
					},
				},
			},
		},
	}

	data, err := block.MarshalJson()
	if err != nil {
		t.Error("Block MarshalJson error")
	}

	var x interface{}
	json.Unmarshal(data, &x)
	fmt.Println(x)

	newBlock := new(Block)
	err = newBlock.UnmarshalJson(data)
	if err != nil {
		t.Error("Block UnmarshalJson error")
	}
	fmt.Println(block.Hash(), "\n", newBlock.Hash())
}
