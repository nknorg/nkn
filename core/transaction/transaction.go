package transaction

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/contract"
	"github.com/nknorg/nkn/core/contract/program"
	sig "github.com/nknorg/nkn/core/signature"
	"github.com/nknorg/nkn/core/transaction/payload"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/util/log"
)

//for different transaction types with different payload format
//and transaction process methods
type TransactionType byte

const (
	Coinbase      TransactionType = 0x00
	TransferAsset TransactionType = 0x10
	RegisterAsset TransactionType = 0x11
	IssueAsset    TransactionType = 0x12
	BookKeeper    TransactionType = 0x20
	Prepaid       TransactionType = 0x40
	Withdraw      TransactionType = 0x41
	Commit        TransactionType = 0x42
	RegisterName  TransactionType = 0x50
	TransferName  TransactionType = 0x51
	DeleteName    TransactionType = 0x52
)

type TransactionResult map[Uint256]Fixed64

//Payload define the func for loading the payload data
//base on payload type which have different struture
type Payload interface {
	//  Get payload data
	Data(version byte) []byte

	//Serialize payload data
	Serialize(w io.Writer, version byte) error

	Deserialize(r io.Reader, version byte) error

	MarshalJson() ([]byte, error)

	UnmarshalJson(data []byte) error
}

//Transaction is used for carry information or action to Ledger
//validated transaction will be added to block and updates state correspondingly

var Store TxnStore

type Transaction struct {
	TxType         TransactionType
	PayloadVersion byte
	Payload        Payload
	Attributes     []*TxnAttribute
	Inputs         []*TxnInput
	Outputs        []*TxnOutput
	Programs       []*program.Program

	hash *Uint256
}

//Serialize the Transaction
func (tx *Transaction) Serialize(w io.Writer) error {

	err := tx.SerializeUnsigned(w)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Transaction txSerializeUnsigned Serialize failed.")
	}
	//Serialize  Transaction's programs
	lens := uint64(len(tx.Programs))
	err = serialization.WriteVarUint(w, lens)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Transaction WriteVarUint failed.")
	}
	if lens > 0 {
		for _, p := range tx.Programs {
			err = p.Serialize(w)
			if err != nil {
				return NewDetailErr(err, ErrNoCode, "Transaction Programs Serialize failed.")
			}
		}
	}
	return nil
}

//Serialize the Transaction data without contracts
func (tx *Transaction) SerializeUnsigned(w io.Writer) error {
	//txType
	w.Write([]byte{byte(tx.TxType)})
	//PayloadVersion
	w.Write([]byte{tx.PayloadVersion})
	//Payload
	if tx.Payload == nil {
		return errors.New("Transaction Payload is nil.")
	}
	tx.Payload.Serialize(w, tx.PayloadVersion)
	//[]*txAttribute
	err := serialization.WriteVarUint(w, uint64(len(tx.Attributes)))
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Transaction item txAttribute length serialization failed.")
	}
	if len(tx.Attributes) > 0 {
		for _, attr := range tx.Attributes {
			attr.Serialize(w)
		}
	}
	//[]*Inputs
	err = serialization.WriteVarUint(w, uint64(len(tx.Inputs)))
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Transaction item Inputs length serialization failed.")
	}
	if len(tx.Inputs) > 0 {
		for _, utxo := range tx.Inputs {
			utxo.Serialize(w)
		}
	}
	// TODO BalanceInputs
	//[]*Outputs
	err = serialization.WriteVarUint(w, uint64(len(tx.Outputs)))
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Transaction item Outputs length serialization failed.")
	}
	if len(tx.Outputs) > 0 {
		for _, output := range tx.Outputs {
			output.Serialize(w)
		}
	}

	return nil
}

//deserialize the Transaction
func (tx *Transaction) Deserialize(r io.Reader) error {
	// tx deserialize
	err := tx.DeserializeUnsigned(r)
	if err != nil {
		log.Error("Deserialize DeserializeUnsigned:", err)
		return NewDetailErr(err, ErrNoCode, "transaction Deserialize error")
	}

	// tx program
	lens, err := serialization.ReadVarUint(r, 0)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "transaction tx program Deserialize error")
	}

	programHashes := []*program.Program{}
	if lens > 0 {
		for i := 0; i < int(lens); i++ {
			outputHashes := new(program.Program)
			outputHashes.Deserialize(r)
			programHashes = append(programHashes, outputHashes)
		}
		tx.Programs = programHashes
	}
	return nil
}

func (tx *Transaction) DeserializeUnsigned(r io.Reader) error {
	var txType [1]byte
	_, err := io.ReadFull(r, txType[:])
	if err != nil {
		log.Error("DeserializeUnsigned ReadFull:", err)
		return err
	}
	tx.TxType = TransactionType(txType[0])
	return tx.DeserializeUnsignedWithoutType(r)
}

func (tx *Transaction) DeserializeUnsignedWithoutType(r io.Reader) error {
	var payloadVersion [1]byte
	_, err := io.ReadFull(r, payloadVersion[:])
	tx.PayloadVersion = payloadVersion[0]
	if err != nil {
		log.Error("DeserializeUnsignedWithoutType:", err)
		return err
	}

	//payload
	//tx.Payload.Deserialize(r)
	switch tx.TxType {
	case RegisterAsset:
		tx.Payload = new(payload.RegisterAsset)
	case IssueAsset:
		tx.Payload = new(payload.IssueAsset)
	case TransferAsset:
		tx.Payload = new(payload.TransferAsset)
	case Coinbase:
		tx.Payload = new(payload.Coinbase)
	case BookKeeper:
		tx.Payload = new(payload.BookKeeper)
	case Prepaid:
		tx.Payload = new(payload.Prepaid)
	case Withdraw:
		tx.Payload = new(payload.Withdraw)
	case Commit:
		tx.Payload = new(payload.Commit)
	case RegisterName:
		tx.Payload = new(payload.RegisterName)
	case DeleteName:
		tx.Payload = new(payload.DeleteName)
	default:
		return errors.New("[Transaction],invalide transaction type.")
	}
	err = tx.Payload.Deserialize(r, tx.PayloadVersion)
	if err != nil {
		log.Error("tx Payload Deserialize:", err)
		return NewDetailErr(err, ErrNoCode, "Payload Parse error")
	}
	//attributes
	Len, err := serialization.ReadVarUint(r, 0)
	if err != nil {
		log.Error("tx attributes Deserialize:", err)
		return err
	}
	if Len > uint64(0) {
		for i := uint64(0); i < Len; i++ {
			attr := new(TxnAttribute)
			err = attr.Deserialize(r)
			if err != nil {
				return err
			}
			tx.Attributes = append(tx.Attributes, attr)
		}
	}
	//Inputs
	Len, err = serialization.ReadVarUint(r, 0)
	if err != nil {
		log.Error("tx Inputs Deserialize:", err)

		return err
	}
	if Len > uint64(0) {
		for i := uint64(0); i < Len; i++ {
			utxo := new(TxnInput)
			err = utxo.Deserialize(r)
			if err != nil {
				return err
			}
			tx.Inputs = append(tx.Inputs, utxo)
		}
	}
	//TODO balanceInputs
	//Outputs
	Len, err = serialization.ReadVarUint(r, 0)
	if err != nil {
		return err
	}
	if Len > uint64(0) {
		for i := uint64(0); i < Len; i++ {
			output := new(TxnOutput)
			output.Deserialize(r)

			tx.Outputs = append(tx.Outputs, output)
		}
	}
	return nil
}

func (tx *Transaction) GetProgramHashes() ([]Uint160, error) {
	if tx == nil {
		return []Uint160{}, errors.New("[Transaction],GetProgramHashes transaction is nil.")
	}
	hashs := []Uint160{}
	uniqHashes := []Uint160{}
	// add inputUTXO's transaction
	referenceWithUTXO_Output, err := tx.GetReference()
	if err != nil {
		return nil, NewDetailErr(err, ErrNoCode, "[Transaction], GetProgramHashes failed.")
	}
	for _, output := range referenceWithUTXO_Output {
		programHash := output.ProgramHash
		hashs = append(hashs, programHash)
	}
	for _, attribute := range tx.Attributes {
		if attribute.Usage == Script {
			dataHash, err := Uint160ParseFromBytes(attribute.Data)
			if err != nil {
				return nil, NewDetailErr(errors.New("[Transaction], GetProgramHashes err."), ErrNoCode, "")
			}
			hashs = append(hashs, Uint160(dataHash))
		}
	}
	switch tx.TxType {
	case RegisterAsset:
		issuer := tx.Payload.(*payload.RegisterAsset).Issuer
		signatureRedeemScript, err := contract.CreateSignatureRedeemScript(issuer)
		if err != nil {
			return nil, NewDetailErr(err, ErrNoCode, "[Transaction], GetProgramHashes CreateSignatureRedeemScript failed.")
		}

		astHash, err := ToCodeHash(signatureRedeemScript)
		if err != nil {
			return nil, NewDetailErr(err, ErrNoCode, "[Transaction], GetProgramHashes ToCodeHash failed.")
		}
		hashs = append(hashs, astHash)
	case IssueAsset:
		result := tx.GetMergedAssetIDValueFromOutputs()
		if err != nil {
			return nil, NewDetailErr(err, ErrNoCode, "[Transaction], GetTransactionResults failed.")
		}
		for k := range result {
			tx, err := Store.GetTransaction(k)
			if err != nil {
				return nil, NewDetailErr(err, ErrNoCode, fmt.Sprintf("[Transaction], GetTransaction failed With AssetID:=%x", k))
			}
			if tx.TxType != RegisterAsset {
				return nil, NewDetailErr(errors.New("[Transaction] error"), ErrNoCode, fmt.Sprintf("[Transaction], Transaction Type ileage With AssetID:=%x", k))
			}

			switch v1 := tx.Payload.(type) {
			case *payload.RegisterAsset:
				hashs = append(hashs, v1.Controller)
			default:
				return nil, NewDetailErr(errors.New("[Transaction] error"), ErrNoCode, fmt.Sprintf("[Transaction], payload is illegal", k))
			}
		}
	case Prepaid:
	case TransferAsset:
	case Commit:
		hashs = append(hashs, tx.Payload.(*payload.Commit).Submitter)
	case Withdraw:
		hashs = append(hashs, tx.Payload.(*payload.Withdraw).ProgramHash)
	case BookKeeper:
		issuer := tx.Payload.(*payload.BookKeeper).Issuer
		signatureRedeemScript, err := contract.CreateSignatureRedeemScript(issuer)
		if err != nil {
			return nil, NewDetailErr(err, ErrNoCode, "[Transaction - BookKeeper], GetProgramHashes CreateSignatureRedeemScript failed.")
		}

		astHash, err := ToCodeHash(signatureRedeemScript)
		if err != nil {
			return nil, NewDetailErr(err, ErrNoCode, "[Transaction - BookKeeper], GetProgramHashes ToCodeHash failed.")
		}
		hashs = append(hashs, astHash)
	case RegisterName:
		registrant := tx.Payload.(*payload.RegisterName).Registrant
		signatureRedeemScript, err := contract.CreateSignatureRedeemScriptWithEncodedPublicKey(registrant)
		if err != nil {
			return nil, NewDetailErr(err, ErrNoCode, "[Transaction], GetProgramHashes CreateSignatureRedeemScript failed.")
		}

		astHash, err := ToCodeHash(signatureRedeemScript)
		if err != nil {
			return nil, NewDetailErr(err, ErrNoCode, "[Transaction], GetProgramHashes ToCodeHash failed.")
		}
		hashs = append(hashs, astHash)
	case DeleteName:
		registrant := tx.Payload.(*payload.DeleteName).Registrant
		signatureRedeemScript, err := contract.CreateSignatureRedeemScriptWithEncodedPublicKey(registrant)
		if err != nil {
			return nil, NewDetailErr(err, ErrNoCode, "[Transaction], GetProgramHashes CreateSignatureRedeemScript failed.")
		}

		astHash, err := ToCodeHash(signatureRedeemScript)
		if err != nil {
			return nil, NewDetailErr(err, ErrNoCode, "[Transaction], GetProgramHashes ToCodeHash failed.")
		}
		hashs = append(hashs, astHash)
	default:
	}
	//remove dupilicated hashes
	uniq := make(map[Uint160]bool)
	for _, v := range hashs {
		uniq[v] = true
	}
	for k := range uniq {
		uniqHashes = append(uniqHashes, k)
	}
	sort.Sort(byProgramHashes(uniqHashes))
	return uniqHashes, nil
}

func (tx *Transaction) SetPrograms(programs []*program.Program) {
	tx.Programs = programs
}

func (tx *Transaction) GetPrograms() []*program.Program {
	return tx.Programs
}

func (tx *Transaction) GetOutputHashes() ([]Uint160, error) {
	//TODO: implement Transaction.GetOutputHashes()

	return []Uint160{}, nil
}

func (tx *Transaction) GenerateAssetMaps() {
	//TODO: implement Transaction.GenerateAssetMaps()
}

func (tx *Transaction) GetMessage() []byte {
	return sig.GetHashData(tx)
}

func (tx *Transaction) ToArray() []byte {
	b := new(bytes.Buffer)
	tx.Serialize(b)
	return b.Bytes()
}

func (tx *Transaction) Hash() Uint256 {
	if tx.hash == nil {
		d := sig.GetHashData(tx)
		temp := sha256.Sum256([]byte(d))
		f := Uint256(sha256.Sum256(temp[:]))
		tx.hash = &f
	}
	return *tx.hash

}

func (tx *Transaction) SetHash(hash Uint256) {
	tx.hash = &hash
}

func (tx *Transaction) Type() InventoryType {
	return TRANSACTION
}
func (tx *Transaction) Verify() error {
	//TODO: Verify()
	return nil
}

func (tx *Transaction) GetReference() (map[*TxnInput]*TxnOutput, error) {
	if tx.TxType == RegisterAsset {
		return nil, nil
	}
	//UTXO input /  Outputs
	reference := make(map[*TxnInput]*TxnOutput)
	// Key index，v UTXOInput
	for _, utxo := range tx.Inputs {
		transaction, err := Store.GetTransaction(utxo.ReferTxID)
		if err != nil {
			return nil, NewDetailErr(err, ErrNoCode, "[Transaction], GetReference failed.")
		}
		index := utxo.ReferTxOutputIndex
		reference[utxo] = transaction.Outputs[index]
	}
	return reference, nil
}
func (tx *Transaction) GetTransactionResults() (TransactionResult, error) {
	result := make(map[Uint256]Fixed64)
	outputResult := tx.GetMergedAssetIDValueFromOutputs()
	InputResult, err := tx.GetMergedAssetIDValueFromReference()
	if err != nil {
		return nil, err
	}
	//calc the balance of input vs output
	for outputAssetid, outputValue := range outputResult {
		if inputValue, ok := InputResult[outputAssetid]; ok {
			result[outputAssetid] = inputValue - outputValue
		} else {
			result[outputAssetid] -= outputValue
		}
	}
	for inputAssetid, inputValue := range InputResult {
		if _, exist := result[inputAssetid]; !exist {
			result[inputAssetid] += inputValue
		}
	}
	return result, nil
}

func (tx *Transaction) GetMergedAssetIDValueFromOutputs() TransactionResult {
	var result = make(map[Uint256]Fixed64)
	for _, v := range tx.Outputs {
		amout, ok := result[v.AssetID]
		if ok {
			result[v.AssetID] = amout + v.Value
		} else {
			result[v.AssetID] = v.Value
		}
	}
	return result
}

func (tx *Transaction) GetMergedAssetIDValueFromReference() (TransactionResult, error) {
	reference, err := tx.GetReference()
	if err != nil {
		return nil, err
	}
	var result = make(map[Uint256]Fixed64)
	for _, v := range reference {
		amout, ok := result[v.AssetID]
		if ok {
			result[v.AssetID] = amout + v.Value
		} else {
			result[v.AssetID] = v.Value
		}
	}
	return result, nil
}

type byProgramHashes []Uint160

func (a byProgramHashes) Len() int      { return len(a) }
func (a byProgramHashes) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byProgramHashes) Less(i, j int) bool {
	if a[i].CompareTo(a[j]) > 0 {
		return false
	} else {
		return true
	}
}

func (tx *Transaction) MarshalJson() ([]byte, error) {
	var txInfo TransactionInfo

	txInfo.TxType = tx.TxType
	txInfo.PayloadVersion = tx.PayloadVersion

	if tx.Payload != nil {
		info, err := tx.Payload.MarshalJson()
		if err != nil {
			return nil, err
		}
		var t interface{}
		json.Unmarshal(info, &t)

		txInfo.Payload = t
	}

	for _, v := range tx.Attributes {
		info, err := v.MarshalJson()
		if err != nil {
			return nil, err
		}
		var t TxnAttributeInfo
		json.Unmarshal(info, &t)
		txInfo.Attributes = append(txInfo.Attributes, t)
	}

	for _, v := range tx.Inputs {
		info, err := v.MarshalJson()
		if err != nil {
			return nil, err
		}
		var t TxnInputInfo
		json.Unmarshal(info, &t)
		txInfo.Inputs = append(txInfo.Inputs, t)
	}

	for _, v := range tx.Outputs {
		info, err := v.MarshalJson()
		if err != nil {
			return nil, err
		}
		var t TxnOutputInfo
		json.Unmarshal(info, &t)
		txInfo.Outputs = append(txInfo.Outputs, t)
	}

	for _, v := range tx.Programs {
		info, err := v.MarshalJson()
		if err != nil {
			return nil, err
		}
		var t program.ProgramInfo
		json.Unmarshal(info, &t)
		txInfo.Programs = append(txInfo.Programs, t)
	}

	hash := tx.Hash()
	txInfo.Hash = BytesToHexString(hash.ToArrayReverse())

	data, err := json.Marshal(txInfo)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (tx *Transaction) UnmarshalJson(data []byte) error {
	txInfo := new(TransactionInfo)
	var err error
	if err = json.Unmarshal(data, &txInfo); err != nil {
		return err
	}

	tx.TxType = txInfo.TxType
	tx.PayloadVersion = txInfo.PayloadVersion

	info, err := json.Marshal(txInfo.Payload)
	if err != nil {
		return err
	}
	switch tx.TxType {
	case RegisterAsset:
		tx.Payload = new(payload.RegisterAsset)
	case IssueAsset:
		tx.Payload = new(payload.IssueAsset)
	case TransferAsset:
		tx.Payload = new(payload.TransferAsset)
	case Coinbase:
		tx.Payload = new(payload.Coinbase)
	case BookKeeper:
		tx.Payload = new(payload.BookKeeper)
	case Prepaid:
		tx.Payload = new(payload.Prepaid)
	case Withdraw:
		tx.Payload = new(payload.Withdraw)
	case Commit:
		tx.Payload = new(payload.Commit)
	case RegisterName:
		tx.Payload = new(payload.RegisterName)
	case DeleteName:
		tx.Payload = new(payload.DeleteName)
	default:
		return errors.New("[Transaction],invalide transaction type.")
	}
	err = tx.Payload.UnmarshalJson(info)
	if err != nil {
		return err
	}

	for _, v := range txInfo.Attributes {
		info, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var at TxnAttribute
		err = at.UnmarshalJson(info)
		if err != nil {
			return err
		}
		tx.Attributes = append(tx.Attributes, &at)
	}

	for _, v := range txInfo.Inputs {
		info, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var ui TxnInput
		err = ui.UnmarshalJson(info)
		if err != nil {
			return err
		}
		tx.Inputs = append(tx.Inputs, &ui)
	}

	for _, v := range txInfo.Outputs {
		info, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var o TxnOutput
		err = o.UnmarshalJson(info)
		if err != nil {
			return err
		}
		tx.Outputs = append(tx.Outputs, &o)
	}

	for _, v := range txInfo.Programs {
		info, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var p program.Program
		err = p.UnmarshalJson(info)
		if err != nil {
			return err
		}
		tx.Programs = append(tx.Programs, &p)
	}

	if txInfo.Hash != "" {
		hashSlice, err := HexStringToBytesReverse(txInfo.Hash)
		if err != nil {
			return err
		}
		hash, err := Uint256ParseFromBytes(hashSlice)
		if err != nil {
			return err
		}
		tx.hash = &hash
	}

	return nil
}
