package transaction

import (
	"errors"
	"fmt"
	"math"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/asset"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/core/validation"
	"github.com/nknorg/nkn/crypto"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/util/log"
)

type TxnHistory interface {
	GetTransaction(hash Uint256) (*Transaction, error)
	GetQuantityIssued(AssetId Uint256) (Fixed64, error)
	IsDoubleSpend(tx *Transaction) bool
	GetAsset(hash Uint256) (*asset.Asset, error)
	GetBookKeeperList() ([]*crypto.PubKey, []*crypto.PubKey, error)
	GetPrepaidInfo(programHash Uint160) (*Fixed64, *Fixed64, error)
	IsTxHashDuplicate(txhash Uint256) bool
}

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

	if err := CheckTransactionPayload(Tx); err != nil {
		log.Warn("[VerifyTransaction],", err)
		return ErrTransactionPayload
	}

	return ErrNoError
}

// VerifyTransactionWithBlock verifys a transaction with current transaction pool in memory
func VerifyTransactionWithBlock(TxPool []*Transaction) error {
	//initial
	txnlist := make(map[Uint256]*Transaction, 0)
	var txPoolInputs []string
	//sum all inputs in TxPool
	for _, Tx := range TxPool {
		for _, UTXOinput := range Tx.UTXOInputs {
			txPoolInputs = append(txPoolInputs, UTXOinput.ToString())
		}
	}
	//start check
	for _, txn := range TxPool {
		//1.check weather have duplicate transaction.
		if _, exist := txnlist[txn.Hash()]; exist {
			return errors.New("[VerifyTransactionWithBlock], duplicate transaction exist in block.")
		} else {
			txnlist[txn.Hash()] = txn
		}
		//2.check Duplicate Utxo input
		if err := CheckDuplicateUtxoInBlock(txn, txPoolInputs); err != nil {
			return err
		}
		//3.check issue amount
		switch txn.TxType {
		case IssueAsset:
			//TODO: use delta mode to improve performance
			results := txn.GetMergedAssetIDValueFromOutputs()
			for k, _ := range results {
				//Get the Asset amount when RegisterAsseted.
				trx, err := TxStore.GetTransaction(k)
				if trx.TxType != RegisterAsset {
					return errors.New("[VerifyTransaction], TxType is illegal.")
				}
				AssetReg := trx.Payload.(*payload.RegisterAsset)

				//Get the amount has been issued of this assetID
				var quantity_issued Fixed64
				if AssetReg.Amount < Fixed64(0) {
					continue
				} else {
					quantity_issued, err = TxStore.GetQuantityIssued(k)
					if err != nil {
						return errors.New("[VerifyTransaction], GetQuantityIssued failed.")
					}
				}

				//calc the amounts in txPool which are also IssueAsset
				var txPoolAmounts Fixed64
				for _, t := range TxPool {
					if t.TxType == IssueAsset {
						outputResult := t.GetMergedAssetIDValueFromOutputs()
						for txidInPool, txValueInPool := range outputResult {
							if txidInPool == k {
								txPoolAmounts = txPoolAmounts + txValueInPool
							}
						}
					}
				}

				//calc weather out off the amount when Registed.
				//AssetReg.Amount : amount when RegisterAsset of this assedID
				//quantity_issued : amount has been issued of this assedID
				//txPoolAmounts   : amount in transactionPool of this assedID of issue transaction.
				if AssetReg.Amount-quantity_issued < txPoolAmounts {
					return errors.New("[VerifyTransaction], Amount check error.")
				}
			}
		}

	}

	return nil
}

// VerifyTransactionWithLedger verifys a transaction with history transaction in ledger
func VerifyTransactionWithLedger(Tx *Transaction) ErrCode {
	if IsDoubleSpend(Tx) {
		log.Info("[VerifyTransactionWithLedger] IsDoubleSpend check faild.")
		return ErrDoubleSpend
	}
	if exist := TxStore.IsTxHashDuplicate(Tx.Hash()); exist {
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

func CheckDuplicateUtxoInBlock(tx *Transaction, txPoolInputs []string) error {
	var txInputs []string
	for _, t := range tx.UTXOInputs {
		txInputs = append(txInputs, t.ToString())
	}
	for _, i := range txInputs {
		for _, j := range txPoolInputs {
			if i == j {
				return errors.New("Duplicated UTXO inputs found in tx pool")
			}
		}
	}
	return nil
}

func IsDoubleSpend(tx *Transaction) bool {
	return TxStore.IsDoubleSpend(tx)
}

func CheckAssetPrecision(Tx *Transaction) error {
	if len(Tx.Outputs) == 0 {
		return nil
	}
	assetOutputs := make(map[Uint256][]*TxOutput, len(Tx.Outputs))

	for _, v := range Tx.Outputs {
		assetOutputs[v.AssetID] = append(assetOutputs[v.AssetID], v)
	}
	for k, outputs := range assetOutputs {
		asset, err := TxStore.GetAsset(k)
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

func CheckTransactionBalance(txn *Transaction) error {
	if txn.TxType == Prepaid || txn.TxType == Withdraw || txn.TxType == Commit {
		return nil
	}
	for _, v := range txn.Outputs {
		if v.Value <= Fixed64(0) {
			return errors.New("Invalide transaction UTXO output.")
		}
	}
	if txn.TxType == IssueAsset {
		if len(txn.UTXOInputs) > 0 {
			return errors.New("Invalide Issue transaction.")
		}
		return nil
	}
	results, err := txn.GetTransactionResults()
	if err != nil {
		return err
	}
	for k, v := range results {
		if v != 0 {
			log.Debug(fmt.Sprintf("AssetID %x in Transfer transactions %x , Input/output UTXO not equal.", k, txn.Hash()))
			return errors.New(fmt.Sprintf("AssetID %x in Transfer transactions %x , Input/output UTXO not equal.", k, txn.Hash()))
		}
	}
	return nil
}

func CheckAttributeProgram(Tx *Transaction) error {
	//TODO: implement CheckAttributeProgram
	return nil
}

func CheckTransactionContracts(Tx *Transaction) error {
	flag, err := validation.VerifySignableData(Tx)
	if flag && err == nil {
		return nil
	} else {
		return err
	}
}

func checkAmountPrecise(amount Fixed64, precision byte) bool {
	return amount.GetData()%int64(math.Pow(10, 8-float64(precision))) != 0
}

func checkIssuerInBookkeeperList(issuer *crypto.PubKey, bookKeepers []*crypto.PubKey) bool {
	for _, bk := range bookKeepers {
		r := crypto.Equal(issuer, bk)
		if r == true {
			log.Debug("issuer is in bookkeeperlist")
			return true
		}
	}
	log.Debug("issuer is NOT in bookkeeperlist")
	return false
}

func CheckTransactionPayload(txn *Transaction) error {

	switch pld := txn.Payload.(type) {
	case *payload.BookKeeper:
		//Todo: validate bookKeeper Cert
		_ = pld.Cert
		bookKeepers, _, _ := TxStore.GetBookKeeperList()
		r := checkIssuerInBookkeeperList(pld.Issuer, bookKeepers)
		if r == false {
			return errors.New("The issuer isn't bookekeeper, can't add other in bookkeepers list.")
		}
		return nil
	case *payload.RegisterAsset:
		if pld.Asset.Precision < asset.MinPrecision || pld.Asset.Precision > asset.MaxPrecision {
			return errors.New("Invalide asset Precision.")
		}
		if checkAmountPrecise(pld.Amount, pld.Asset.Precision) {
			return errors.New("Invalide asset value,out of precise.")
		}
	case *payload.IssueAsset:
	case *payload.TransferAsset:
	case *payload.BookKeeping:
	case *payload.Commit:
	case *payload.Prepaid:
		var inputAmount, outputAmount Fixed64
		for _, input := range txn.UTXOInputs {
			reftxn, err := TxStore.GetTransaction(input.ReferTxID)
			if err != nil {
				return err
			}
			inputAmount += reftxn.Outputs[input.ReferTxOutputIndex].Value
		}
		for _, output := range txn.Outputs {
			outputAmount += output.Value
		}
		if inputAmount-outputAmount != pld.Amount {
			return errors.New("prepaid transaction balance unmatched")
		}
	case *payload.Withdraw:
		var outputAmount Fixed64

		for _, output := range txn.Outputs {
			outputAmount += output.Value
		}
		prepaidAmount, _, err := TxStore.GetPrepaidInfo(pld.ProgramHash)
		if err != nil {
			return err
		}
		if outputAmount > *prepaidAmount {
			return errors.New("asset is not enough")
		}
	default:
		return errors.New("[txValidator],invalidate transaction payload type.")
	}
	return nil
}
