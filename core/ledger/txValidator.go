package ledger

import (
	"errors"
	"fmt"
	"math"
	"nkn-core/common"
	"nkn-core/common/log"
	. "nkn-core/errors"
)

// VerifyTransaction verifys received single transaction
func VerifyTransaction(Tx *Transaction) ErrCode {

	if err := CheckDuplicateInput(Tx); err != nil {
		log.Warn("[VerifyTransaction],", err)
		return ErrDuplicateInput
	}

	if err := CheckAssetPrecision(Tx); err != nil {
		log.Warn("[VerifyTransaction],", err)
		return ErrAssetPrecision
	}

	if err := CheckTransactionBalance(Tx); err != nil {
		log.Warn("[VerifyTransaction],", err)
		return ErrTransactionBalance
	}

	if err := CheckAttributeProgram(Tx); err != nil {
		log.Warn("[VerifyTransaction],", err)
		return ErrAttributeProgram
	}

	if err := CheckTransactionContracts(Tx); err != nil {
		log.Warn("[VerifyTransaction],", err)
		return ErrTransactionContracts
	}

	return ErrNoError
}

// VerifyTransactionWithLedger verifys a transaction with history transaction in ledger
func VerifyTransactionWithLedger(Tx *Transaction, ledger *Ledger) ErrCode {
	if IsDoubleSpend(Tx, ledger) {
		log.Info("[VerifyTransactionWithLedger] IsDoubleSpend check faild.")
		return ErrDoubleSpend
	}
	if exist := ledger.Store.IsTxHashDuplicate(Tx.Hash()); exist {
		log.Info("[VerifyTransactionWithLedger] duplicate transaction check faild.")
		return ErrTxHashDuplicate
	}
	return ErrNoError
}

//validate the transaction of duplicate UTXO input
func CheckDuplicateInput(tx *Transaction) error {
	if len(tx.UTXOInputs) == 0 {
		return nil
	}
	for i, utxoin := range tx.UTXOInputs {
		for j := 0; j < i; j++ {
			if utxoin.ReferTxID == tx.UTXOInputs[j].ReferTxID && utxoin.ReferTxOutputIndex == tx.UTXOInputs[j].ReferTxOutputIndex {
				return errors.New("invalid transaction")
			}
		}
	}
	return nil
}

func IsDoubleSpend(tx *Transaction, ledger *Ledger) bool {
	return ledger.IsDoubleSpend(tx)
}

func CheckAssetPrecision(Tx *Transaction) error {
	if len(Tx.Outputs) == 0 {
		return nil
	}
	assetOutputs := make(map[common.Uint256][]*TxOutput, len(Tx.Outputs))

	for _, v := range Tx.Outputs {
		assetOutputs[v.AssetID] = append(assetOutputs[v.AssetID], v)
	}
	for k, outputs := range assetOutputs {
		asset, err := DefaultLedger.GetAsset(k)
		if err != nil {
			return errors.New("The asset not exist in local blockchain.")
		}
		precision := asset.Precision
		for _, output := range outputs {
			if checkAmountPrecise(output.Value, precision) {
				return errors.New("The precision of asset is incorrect.")
			}
		}
	}
	return nil
}

func CheckTransactionBalance(Tx *Transaction) error {
	for _, v := range Tx.Outputs {
		if v.Value <= common.Fixed64(0) {
			return errors.New("Invalide transaction UTXO output.")
		}
	}
	results, err := Tx.GetTransactionResults()
	if err != nil {
		return err
	}
	for k, v := range results {
		if v != 0 {
			log.Debug(fmt.Sprintf("AssetID %x in Transfer transactions %x , Input/output UTXO not equal.", k, Tx.Hash()))
			return errors.New(fmt.Sprintf("AssetID %x in Transfer transactions %x , Input/output UTXO not equal.", k, Tx.Hash()))
		}
	}
	return nil
}

func CheckAttributeProgram(Tx *Transaction) error {
	//TODO: implement CheckAttributeProgram
	return nil
}

func CheckTransactionContracts(Tx *Transaction) error {
	flag, err := VerifySignableData(Tx)
	if flag && err == nil {
		return nil
	} else {
		return err
	}
}

func checkAmountPrecise(amount common.Fixed64, precision byte) bool {
	return amount.GetData()%int64(math.Pow(10, 8-float64(precision))) != 0
}
