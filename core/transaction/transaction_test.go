package transaction

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/contract/program"
	"github.com/nknorg/nkn/core/transaction/payload"
)

func TestTxAttribute(t *testing.T) {
	ta := &TxnAttribute{
		Usage: 1,
		Data:  []byte{1, 2, 3, 4},
		Size:  5,
	}
	data, err := ta.MarshalJson()
	if err != nil {
		t.Error("TxnAttribute MarshalJson error")
	}
	txa := new(TxnAttribute)
	err = txa.UnmarshalJson(data)
	if err != nil {
		t.Error("TxnAttribute unmarshalJson error")
	}

	if !txa.Equal(ta) {
		t.Log(ta, txa)
		t.Error("TxnAttribute compare error")
	}
}

func TestUTXOTxInput(t *testing.T) {
	ui := &TxnInput{
		ReferTxID:          common.Uint256{0, 1, 2, 4},
		ReferTxOutputIndex: 5,
	}
	data, err := ui.MarshalJson()
	if err != nil {
		t.Error("TxnInput MarshalJson error")
	}

	input := new(TxnInput)
	err = input.UnmarshalJson(data)
	if err != nil {
		t.Error("TestUTXOTxInput unmarshalJson error")
	}

	if !input.Equal(ui) {
		t.Log(input, ui)
		t.Error("TestUTXOTxInput compare error")
	}
}

func TestTxOutput(t *testing.T) {
	to := &TxnOutput{
		AssetID:     common.Uint256{0, 1, 2, 3, 4},
		Value:       common.Fixed64(314159260),
		ProgramHash: common.Uint160{1, 2, 3, 4, 5, 6, 7, 8},
	}
	data, err := to.MarshalJson()
	if err != nil {
		t.Error("TxnOutput MarshalJson error")
	}

	output := new(TxnOutput)
	err = output.UnmarshalJson(data)
	if err != nil {
		t.Error("TxnOutput unmarshalJson error")
	}

	if !output.Equal(to) {
		t.Log(output, to)
		t.Error("TestTxOutput compare error")
	}
}

func TestTransaction(t *testing.T) {
	tx := &Transaction{
		TxType:         0x42,
		PayloadVersion: 1,
		Payload: &payload.Commit{
			SigChain:  []byte{1, 2, 3, 4},
			Submitter: common.Uint160{5, 6, 7, 8},
		},
		Attributes: []*TxnAttribute{
			&TxnAttribute{
				Usage: 1,
				Data:  []byte{1, 2, 3, 4},
				Size:  5,
			},
			&TxnAttribute{
				Usage: 1,
				Data:  []byte{3, 6, 2, 1},
			},
		},
		Inputs: []*TxnInput{
			&TxnInput{
				ReferTxID:          common.Uint256{0, 1, 2, 4},
				ReferTxOutputIndex: 5,
			},
			&TxnInput{
				ReferTxID:          common.Uint256{6, 7},
				ReferTxOutputIndex: 3,
			},
		},
		Outputs: []*TxnOutput{
			&TxnOutput{
				AssetID:     common.Uint256{0, 1, 2, 3, 4},
				Value:       common.Fixed64(314159260),
				ProgramHash: common.Uint160{1, 2, 3, 4, 5, 6, 7, 8},
			},
			&TxnOutput{
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
		//hash: &common.Uint256{1, 2, 3, 5},
	}

	data, err := tx.MarshalJson()
	if err != nil {
		t.Error("Transacion MarshalJson error")
	}

	var x interface{}
	json.Unmarshal(data, &x)

	txn := new(Transaction)
	err = txn.UnmarshalJson(data)
	if err != nil {
		t.Error("Transacion MarshalJson error")
	}

	txnHash := txn.Hash()
	if (&txnHash).CompareTo(tx.Hash()) != 0 {
		t.Error("Transaction compare error")
	}

	fmt.Println(txnHash, tx.Hash())
}

func TestTransaction2(t *testing.T) {
	tx := &Transaction{
		TxType:         0x10,
		PayloadVersion: 1,
		Payload:        &payload.TransferAsset{},
		Attributes: []*TxnAttribute{
			&TxnAttribute{
				Usage: 1,
				Data:  []byte{1, 2, 3, 4},
				Size:  5,
			},
			&TxnAttribute{
				Usage: 1,
				Data:  []byte{3, 6, 2, 1},
			},
		},
		Inputs: []*TxnInput{
			&TxnInput{
				ReferTxID:          common.Uint256{0, 1, 2, 4},
				ReferTxOutputIndex: 5,
			},
			&TxnInput{
				ReferTxID:          common.Uint256{6, 7},
				ReferTxOutputIndex: 3,
			},
		},
		Outputs: []*TxnOutput{
			&TxnOutput{
				AssetID:     common.Uint256{0, 1, 2, 3, 4},
				Value:       common.Fixed64(314159260),
				ProgramHash: common.Uint160{1, 2, 3, 4, 5, 6, 7, 8},
			},
			&TxnOutput{
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
		//hash: &common.Uint256{1, 2, 3, 5},
	}

	data, err := tx.MarshalJson()
	if err != nil {
		t.Error("Transacion MarshalJson error")
	}

	var x interface{}
	json.Unmarshal(data, &x)

	txn := new(Transaction)
	err = txn.UnmarshalJson(data)
	if err != nil {
		t.Error("Transacion MarshalJson error")
	}

	txnHash := txn.Hash()
	if txnHash.CompareTo(tx.Hash()) != 0 {
		t.Error("Transaction compare error")
	}
	fmt.Println(txnHash, tx.Hash())
}
