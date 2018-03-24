package service

import (
	"nkn-core/common"
	"nkn-core/core/contract"
	"nkn-core/core/ledger"
	"nkn-core/core/signature"
	"nkn-core/core/transaction"
	"nkn-core/crypto"
	"nkn-core/errors"
	"nkn-core/smartcontract/states"
	"nkn-core/vm/avm"
	"nkn-core/vm/avm/types"
	"fmt"
	"math/big"
)

type StateReader struct {
	serviceMap map[string]func(*avm.ExecutionEngine) (bool, error)
}

func NewStateReader() *StateReader {
	var stateReader StateReader
	stateReader.serviceMap = make(map[string]func(*avm.ExecutionEngine) (bool, error), 0)
	stateReader.Register("Neo.Runtime.GetTrigger", stateReader.RuntimeGetTrigger)
	stateReader.Register("Neo.Runtime.CheckWitness", stateReader.RuntimeCheckWitness)
	stateReader.Register("Neo.Runtime.Notify", stateReader.RuntimeNotify)
	stateReader.Register("Neo.Runtime.Log", stateReader.RuntimeLog)

	stateReader.Register("Neo.Blockchain.GetHeight", stateReader.BlockChainGetHeight)
	stateReader.Register("Neo.Blockchain.GetHeader", stateReader.BlockChainGetHeader)
	stateReader.Register("Neo.Blockchain.GetBlock", stateReader.BlockChainGetBlock)
	stateReader.Register("Neo.Blockchain.GetTransaction", stateReader.BlockChainGetTransaction)
	stateReader.Register("Neo.Blockchain.GetAccount", stateReader.BlockChainGetAccount)
	stateReader.Register("Neo.Blockchain.GetValidators", stateReader.BlockChainGetValidators)
	stateReader.Register("Neo.Blockchain.GetAsset", stateReader.BlockChainGetAsset)

	stateReader.Register("Neo.Header.GetHash", stateReader.HeaderGetHash)
	stateReader.Register("Neo.Header.GetVersion", stateReader.HeaderGetVersion)
	stateReader.Register("Neo.Header.GetPrevHash", stateReader.HeaderGetPrevHash)
	stateReader.Register("Neo.Header.GetMerkleRoot", stateReader.HeaderGetMerkleRoot)
	stateReader.Register("Neo.Header.GetTimestamp", stateReader.HeaderGetTimestamp)
	stateReader.Register("Neo.Header.GetConsensusData", stateReader.HeaderGetConsensusData)
	stateReader.Register("Neo.Header.GetNextConsensus", stateReader.HeaderGetNextConsensus)

	stateReader.Register("Neo.Block.GetTransactionCount", stateReader.BlockGetTransactionCount)
	stateReader.Register("Neo.Block.GetTransactions", stateReader.BlockGetTransactions)
	stateReader.Register("Neo.Block.GetTransaction", stateReader.BlockGetTransaction)

	stateReader.Register("Neo.Transaction.GetHash", stateReader.TransactionGetHash)
	stateReader.Register("Neo.Transaction.GetType", stateReader.TransactionGetType)
	stateReader.Register("Neo.Transaction.GetAttributes", stateReader.TransactionGetAttributes)
	stateReader.Register("Neo.Transaction.GetInputs", stateReader.TransactionGetInputs)
	stateReader.Register("Neo.Transaction.GetOutputs", stateReader.TransactionGetOutputs)
	stateReader.Register("Neo.Transaction.GetReferences", stateReader.TransactionGetReferences)

	stateReader.Register("Neo.Attribute.GetUsage", stateReader.AttributeGetUsage)
	stateReader.Register("Neo.Attribute.GetData", stateReader.AttributeGetData)

	stateReader.Register("Neo.Input.GetHash", stateReader.InputGetHash)
	stateReader.Register("Neo.Input.GetIndex", stateReader.InputGetIndex)

	stateReader.Register("Neo.Output.GetAssetId", stateReader.OutputGetAssetId)
	stateReader.Register("Neo.Output.GetValue", stateReader.OutputGetValue)
	stateReader.Register("Neo.Output.GetScriptHash", stateReader.OutputGetCodeHash)

	stateReader.Register("Neo.Account.GetScriptHash", stateReader.AccountGetCodeHash)
	stateReader.Register("Neo.Account.GetBalance", stateReader.AccountGetBalance)

	stateReader.Register("Neo.Asset.GetAssetId", stateReader.AssetGetAssetId)
	stateReader.Register("Neo.Asset.GetAssetType", stateReader.AssetGetAssetType)
	stateReader.Register("Neo.Asset.GetAmount", stateReader.AssetGetAmount)
	stateReader.Register("Neo.Asset.GetAvailable", stateReader.AssetGetAvailable)
	stateReader.Register("Neo.Asset.GetPrecision", stateReader.AssetGetPrecision)
	stateReader.Register("Neo.Asset.GetOwner", stateReader.AssetGetOwner)
	stateReader.Register("Neo.Asset.GetAdmin", stateReader.AssetGetAdmin)
	stateReader.Register("Neo.Asset.GetIssuer", stateReader.AssetGetIssuer)

	stateReader.Register("Neo.Contract.GetScript", stateReader.ContractGetCode)

	stateReader.Register("Neo.Storage.GetContext", stateReader.StorageGetContext)

	return &stateReader
}

func (s *StateReader) Register(methodName string, handler func(*avm.ExecutionEngine) (bool, error)) bool {
	if _, ok := s.serviceMap[methodName]; ok {
		return false
	}
	s.serviceMap[methodName] = handler
	return true
}

func (s *StateReader) GetServiceMap() map[string]func(*avm.ExecutionEngine) (bool, error) {
	return s.serviceMap
}

func (s *StateReader) RuntimeGetTrigger(e *avm.ExecutionEngine) (bool, error) {
	return true, nil
}

func (s *StateReader) RuntimeNotify(e *avm.ExecutionEngine) (bool, error) {
	avm.PopStackItem(e)
	return true, nil
}

func (s *StateReader) RuntimeLog(e *avm.ExecutionEngine) (bool, error) {
	return true, nil
}

func (s *StateReader) CheckWitnessHash(engine *avm.ExecutionEngine, programHash common.Uint160) (bool, error) {
	hashForVerifying, err := engine.GetCodeContainer().(signature.SignableData).GetProgramHashes()
	if err != nil {
		return false, err
	}
	return contains(hashForVerifying, programHash), nil
}

func (s *StateReader) CheckWitnessPublicKey(engine *avm.ExecutionEngine, publicKey *crypto.PubKey) (bool, error) {
	c, err := contract.CreateSignatureRedeemScript(publicKey)
	if err != nil {
		return false, err
	}
	h, err := common.ToCodeHash(c)
	if err != nil {
		return false, err
	}
	return s.CheckWitnessHash(engine, h)
}

func (s *StateReader) RuntimeCheckWitness(e *avm.ExecutionEngine) (bool, error) {
	data := avm.PopByteArray(e)
	var (
		result bool
		err    error
	)
	if len(data) == 20 {
		program, err := common.Uint160ParseFromBytes(data)
		if err != nil {
			return false, err
		}
		result, err = s.CheckWitnessHash(e, program)
	} else if len(data) == 33 {
		publicKey, err := crypto.DecodePoint(data)
		if err != nil {
			return false, err
		}
		result, err = s.CheckWitnessPublicKey(e, publicKey)
	} else {
		return false, errors.NewDetailErr(err, errors.ErrNoCode, "[RuntimeCheckWitness] data invalid.")
	}
	if err != nil {
		return false, err
	}
	avm.PushData(e, result)
	return true, nil
}

func (s *StateReader) BlockChainGetHeight(e *avm.ExecutionEngine) (bool, error) {
	var i uint32
	if ledger.DefaultLedger == nil {
		i = 0
	} else {
		i = ledger.DefaultLedger.Store.GetHeight()
	}
	avm.PushData(e, i)
	return true, nil
}

func (s *StateReader) BlockChainGetHeader(e *avm.ExecutionEngine) (bool, error) {
	data := avm.PopByteArray(e)
	var (
		header *ledger.Header
		err    error
	)
	l := len(data)
	if l <= 5 {
		b := new(big.Int)
		height := uint32(b.SetBytes(common.ToArrayReverse(data)).Int64())
		if ledger.DefaultLedger != nil {
			hash, err := ledger.DefaultLedger.Store.GetBlockHash(height)
			if err != nil {
				return false, err
			}
			header, err = ledger.DefaultLedger.Store.GetHeader(hash)
		}
	} else if l == 32 {
		hash, _ := common.Uint256ParseFromBytes(data)
		if ledger.DefaultLedger != nil {
			header, err = ledger.DefaultLedger.Store.GetHeader(hash)
		}
	} else {
		return false, errors.NewErr("[BlockChainGetHeader] data invalid.")
	}
	if err != nil {
		return false, err
	}
	avm.PushData(e, header)
	return true, nil
}

func (s *StateReader) BlockChainGetBlock(e *avm.ExecutionEngine) (bool, error) {
	data := avm.PopByteArray(e)
	var (
		block *ledger.Block
		err   error
	)
	l := len(data)
	if l <= 5 {
		b := new(big.Int)
		height := uint32(b.SetBytes(common.ToArrayReverse(data)).Int64())
		if ledger.DefaultLedger != nil {
			hash, err := ledger.DefaultLedger.Store.GetBlockHash(height)
			if err != nil {
				return false, err
			}
			block, err = ledger.DefaultLedger.Store.GetBlock(hash)
		}
	} else if l == 32 {
		hash, err := common.Uint256ParseFromBytes(data)
		if err != nil {
			return false, err
		}
		if ledger.DefaultLedger != nil {
			block, err = ledger.DefaultLedger.Store.GetBlock(hash)
		}
	} else {
		return false, errors.NewErr("[BlockChainGetBlock] data invalid.")
	}
	if err != nil {
		return false, err
	}
	avm.PushData(e, block)
	return true, nil
}

func (s *StateReader) BlockChainGetTransaction(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopByteArray(e)
	hash, err := common.Uint256ParseFromBytes(d)
	if err != nil {
		return false, err
	}
	tx, err := ledger.DefaultLedger.Store.GetTransaction(hash)
	if err != nil {
		return false, err
	}

	avm.PushData(e, tx)
	return true, nil
}

func (s *StateReader) BlockChainGetAccount(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopByteArray(e)
	hash, err := common.Uint160ParseFromBytes(d)
	if err != nil {
		return false, err
	}
	account, err := ledger.DefaultLedger.Store.GetAccount(hash)
	avm.PushData(e, account)
	return true, nil
}

func (s *StateReader) BlockChainGetAsset(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopByteArray(e)
	hash, err := common.Uint256ParseFromBytes(d)
	if err != nil {
		return false, err
	}
	assetState, err := ledger.DefaultLedger.Store.GetAssetState(hash)
	if err != nil {
		return false, err
	}
	avm.PushData(e, assetState)
	return true, nil
}

func (s *StateReader) BlockChainGetValidators(e *avm.ExecutionEngine) (bool, error) {
	bookKeeperList, _, err := ledger.DefaultLedger.Store.GetBookKeeperList()
	if err != nil {
		return false, err
	}
	pkList := make([]types.StackItemInterface, 0)
	for _, v := range bookKeeperList {
		pk, err := v.EncodePoint(true)
		if err != nil {
			return false, err
		}
		pkList = append(pkList, types.NewByteArray(pk))
	}
	avm.PushData(e, pkList)
	return true, nil
}

func (s *StateReader) HeaderGetHash(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get header error in function headergethash!")
	}
	h := d.(*ledger.Header).Hash()
	avm.PushData(e, h.ToArray())
	return true, nil
}

func (s *StateReader) HeaderGetVersion(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get header error in function headergetversion")
	}
	version := d.(*ledger.Header).Version
	avm.PushData(e, version)
	return true, nil
}

func (s *StateReader) HeaderGetPrevHash(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get header error in function HeaderGetPrevHash")
	}
	preHash := d.(*ledger.Header).PrevBlockHash
	avm.PushData(e, preHash.ToArray())
	return true, nil
}

func (s *StateReader) HeaderGetMerkleRoot(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get header error in function HeaderGetMerkleRoot")
	}
	root := d.(*ledger.Header).TransactionsRoot
	avm.PushData(e, root.ToArray())
	return true, nil
}

func (s *StateReader) HeaderGetTimestamp(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get header error in function HeaderGetTimestamp")
	}
	timeStamp := d.(*ledger.Header).Timestamp
	avm.PushData(e, timeStamp)
	return true, nil
}

func (s *StateReader) HeaderGetConsensusData(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get header error in function HeaderGetConsensusData")
	}
	consensusData := d.(*ledger.Header).ConsensusData
	avm.PushData(e, consensusData)
	return true, nil
}

func (s *StateReader) HeaderGetNextConsensus(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get header error in function HeaderGetNextConsensus")
	}
	nextBookKeeper := d.(*ledger.Header).NextBookKeeper
	avm.PushData(e, nextBookKeeper.ToArray())
	return true, nil
}

func (s *StateReader) BlockGetTransactionCount(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get block error in function BlockGetTransactionCount")
	}
	transactions := d.(*ledger.Block).Transactions
	avm.PushData(e, len(transactions))
	return true, nil
}

func (s *StateReader) BlockGetTransactions(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get block data error in function BlockGetTransactions")
	}
	transactions := d.(*ledger.Block).Transactions
	transactionList := make([]types.StackItemInterface, 0)
	for _, v := range transactions {
		transactionList = append(transactionList, types.NewInteropInterface(v))
	}
	avm.PushData(e, transactionList)
	return true, nil
}

func (s *StateReader) BlockGetTransaction(e *avm.ExecutionEngine) (bool, error) {
	index := avm.PopInt(e)
	if index < 0 {
		return false, fmt.Errorf("%v", "index invalid in function BlockGetTransaction")
	}
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get transaction data error in function BlockGetTransaction")
	}
	transactions := d.(*ledger.Block).Transactions
	if index >= len(transactions) {
		return false, fmt.Errorf("%v", "index over transaction length in function BlockGetTransaction")
	}
	avm.PushData(e, transactions[index])
	return true, nil
}

func (s *StateReader) TransactionGetHash(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get transaction error in function TransactionGetHash")
	}
	txHash := d.(*transaction.Transaction).Hash()
	avm.PushData(e, txHash.ToArray())
	return true, nil
}

func (s *StateReader) TransactionGetType(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get transaction error in function TransactionGetType")
	}
	txType := d.(*transaction.Transaction).TxType
	avm.PushData(e, int(txType))
	return true, nil
}

func (s *StateReader) TransactionGetAttributes(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get transaction error in function TransactionGetAttributes")
	}
	attributes := d.(*transaction.Transaction).Attributes
	attributList := make([]types.StackItemInterface, 0)
	for _, v := range attributes {
		attributList = append(attributList, types.NewInteropInterface(v))
	}
	avm.PushData(e, attributList)
	return true, nil
}

func (s *StateReader) TransactionGetInputs(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get transaction error in function TransactionGetInputs")
	}
	inputs := d.(*transaction.Transaction).UTXOInputs
	inputList := make([]types.StackItemInterface, 0)
	for _, v := range inputs {
		inputList = append(inputList, types.NewInteropInterface(v))
	}
	avm.PushData(e, inputList)
	return true, nil
}

func (s *StateReader) TransactionGetOutputs(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get transaction error in function TransactionGetOutputs")
	}
	outputs := d.(*transaction.Transaction).Outputs
	outputList := make([]types.StackItemInterface, 0)
	for _, v := range outputs {
		outputList = append(outputList, types.NewInteropInterface(v))
	}
	avm.PushData(e, outputList)
	return true, nil
}

func (s *StateReader) TransactionGetReferences(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get transaction error in function TransactionGetReferences")
	}
	references, err := d.(*transaction.Transaction).GetReference()
	referenceList := make([]types.StackItemInterface, 0)
	for _, v := range references {
		referenceList = append(referenceList, types.NewInteropInterface(v))
	}
	avm.PushData(e, referenceList)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *StateReader) AttributeGetUsage(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get Attribute error in function AttributeGetUsage")
	}
	attribute := d.(*transaction.TxAttribute)
	avm.PushData(e, int(attribute.Usage))
	return true, nil
}

func (s *StateReader) AttributeGetData(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get Attribute error in function AttributeGetUsage")
	}
	attribute := d.(*transaction.TxAttribute)
	avm.PushData(e, attribute.Data)
	return true, nil
}

func (s *StateReader) InputGetHash(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get UTXOTxInput error in function InputGetHash")
	}
	input := d.(*transaction.UTXOTxInput)
	avm.PushData(e, input.ReferTxID.ToArray())
	return true, nil
}

func (s *StateReader) InputGetIndex(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get transaction error in function TransactionGetReferences")
	}
	input := d.(*transaction.UTXOTxInput)
	avm.PushData(e, input.ReferTxOutputIndex)
	return true, nil
}

func (s *StateReader) OutputGetAssetId(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get TxOutput error in function OutputGetAssetId")
	}
	output := d.(*transaction.TxOutput)
	avm.PushData(e, output.AssetID.ToArray())
	return true, nil
}

func (s *StateReader) OutputGetValue(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get TxOutput error in function OutputGetValue")
	}
	output := d.(*transaction.TxOutput)
	avm.PushData(e, output.Value.GetData())
	return true, nil
}

func (s *StateReader) OutputGetCodeHash(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get TxOutput error in function OutputGetCodeHash")
	}
	output := d.(*transaction.TxOutput)
	avm.PushData(e, output.ProgramHash.ToArray())
	return true, nil
}

func (s *StateReader) AccountGetCodeHash(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get AccountState error in function AccountGetCodeHash")
	}
	accountState := d.(*states.AccountState).ProgramHash
	avm.PushData(e, accountState.ToArray())
	return true, nil
}

func (s *StateReader) AccountGetBalance(e *avm.ExecutionEngine) (bool, error) {
	assetIdByte := avm.PopByteArray(e)
	assetId, err := common.Uint256ParseFromBytes(assetIdByte)
	if err != nil {
		return false, err
	}
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get AccountState error in function AccountGetCodeHash")
	}
	accountState := d.(*states.AccountState)
	balance := common.Fixed64(0)
	if v, ok := accountState.Balances[assetId]; ok {
		balance = v
	}
	avm.PushData(e, balance.GetData())
	return true, nil
}

func (s *StateReader) AssetGetAssetId(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get AssetState error in function AssetGetAssetId")
	}
	assetState := d.(*states.AssetState)
	avm.PushData(e, assetState.AssetId.ToArray())
	return true, nil
}

func (s *StateReader) AssetGetAssetType(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get AssetState error in function AssetGetAssetType")
	}
	assetState := d.(*states.AssetState)
	avm.PushData(e, int(assetState.AssetType))
	return true, nil
}

func (s *StateReader) AssetGetAmount(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get AssetState error in function AssetGetAmount")
	}
	assetState := d.(*states.AssetState)
	avm.PushData(e, assetState.Amount.GetData())
	return true, nil
}

func (s *StateReader) AssetGetAvailable(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get AssetState error in function AssetGetAvailable")
	}
	assetState := d.(*states.AssetState)
	avm.PushData(e, assetState.Available.GetData())
	return true, nil
}

func (s *StateReader) AssetGetPrecision(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get AssetState error in function AssetGetPrecision")
	}
	assetState := d.(*states.AssetState)
	avm.PushData(e, int(assetState.Precision))
	return true, nil
}

func (s *StateReader) AssetGetOwner(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get AssetState error in function AssetGetOwner")
	}
	assetState := d.(*states.AssetState)
	owner, err := assetState.Owner.EncodePoint(true)
	if err != nil {
		return false, err
	}
	avm.PushData(e, owner)
	return true, nil
}

func (s *StateReader) AssetGetAdmin(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get AssetState error in function AssetGetAdmin")
	}
	assetState := d.(*states.AssetState)
	avm.PushData(e, assetState.Admin.ToArray())
	return true, nil
}

func (s *StateReader) AssetGetIssuer(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get AssetState error in function AssetGetIssuer")
	}
	assetState := d.(*states.AssetState)
	avm.PushData(e, assetState.Issuer.ToArray())
	return true, nil
}

func (s *StateReader) ContractGetCode(e *avm.ExecutionEngine) (bool, error) {
	d := avm.PopInteropInterface(e)
	if d == nil {
		return false, fmt.Errorf("%v", "Get ContractState error in function ContractGetCode")
	}
	assetState := d.(*states.ContractState)
	avm.PushData(e, assetState.Code.Code)
	return true, nil
}

func (s *StateReader) StorageGetContext(e *avm.ExecutionEngine) (bool, error) {
	codeHash, err := common.Uint160ParseFromBytes(e.CurrentContext().GetCodeHash())
	if err != nil {
		return false, err
	}
	avm.PushData(e, NewStorageContext(&codeHash))
	return true, nil
}
