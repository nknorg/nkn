package service

import (
	"DNA/common"
	"DNA/core/asset"
	"DNA/core/code"
	"DNA/core/contract"
	"DNA/core/ledger"
	"DNA/core/store"
	"DNA/core/transaction"
	"DNA/crypto"
	"DNA/errors"
	. "DNA/smartcontract/errors"
	"DNA/smartcontract/states"
	"DNA/smartcontract/storage"
	"DNA/vm/avm"
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
)

type StateMachine struct {
	*StateReader
	CloneCache *storage.CloneCache
}

func NewStateMachine(dbCache storage.DBCache, innerCache storage.DBCache) *StateMachine {
	var stateMachine StateMachine
	stateMachine.CloneCache = storage.NewCloneDBCache(innerCache, dbCache)
	stateMachine.StateReader = NewStateReader()
	stateMachine.StateReader.Register("Neo.Validator.Register", stateMachine.RegisterValidator)
	stateMachine.StateReader.Register("Neo.Asset.Create", stateMachine.CreateAsset)
	stateMachine.StateReader.Register("Neo.Contract.Create", stateMachine.CreateContract)
	stateMachine.StateReader.Register("Neo.Blockchain.GetContract", stateMachine.GetContract)
	stateMachine.StateReader.Register("Neo.Asset.Renew", stateMachine.AssetRenew)
	stateMachine.StateReader.Register("Neo.Storage.Get", stateMachine.StorageGet)
	stateMachine.StateReader.Register("Neo.Contract.Destroy", stateMachine.ContractDestory)
	stateMachine.StateReader.Register("Neo.Storage.Put", stateMachine.StoragePut)
	stateMachine.StateReader.Register("Neo.Storage.Delete", stateMachine.StorageDelete)
	stateMachine.StateReader.Register("Neo.Contract.GetStorageContext", stateMachine.GetStorageContext)
	return &stateMachine
}

func (s *StateMachine) RegisterValidator(engine *avm.ExecutionEngine) (bool, error) {
	pubkeyByte := avm.PopByteArray(engine)
	pubkey, err := crypto.DecodePoint(pubkeyByte)
	if err != nil {
		return false, err
	}
	if result, err := s.StateReader.CheckWitnessPublicKey(engine, pubkey); !result {
		return result, err
	}
	b := new(bytes.Buffer)
	pubkey.Serialize(b)
	validatorState, err := s.CloneCache.GetInnerCache().GetOrAdd(store.ST_Validator, b.String(), &states.ValidatorState{PublicKey: pubkey})
	if err != nil {
		return false, err
	}
	avm.PushData(engine, validatorState)
	return true, nil
}

func (s *StateMachine) CreateAsset(engine *avm.ExecutionEngine) (bool, error) {
	tx := engine.GetCodeContainer().(*transaction.Transaction)
	assetId := tx.Hash()
	assertType := asset.AssetType(avm.PopInt(engine))
	name := avm.PopByteArray(engine)
	if len(name) > 1024 {
		return false, ErrAssetNameInvalid
	}
	amount := avm.PopBigInt(engine)
	if amount.Int64() == 0 {
		return false, ErrAssetAmountInvalid
	}
	precision := avm.PopBigInt(engine)
	if precision.Int64() > 8 {
		return false, ErrAssetPrecisionInvalid
	}
	if amount.Int64()%int64(math.Pow(10, 8-float64(precision.Int64()))) != 0 {
		return false, ErrAssetAmountInvalid
	}
	ownerByte := avm.PopByteArray(engine)
	owner, err := crypto.DecodePoint(ownerByte)
	if err != nil {
		return false, err
	}
	if result, err := s.StateReader.CheckWitnessPublicKey(engine, owner); !result {
		return result, err
	}
	adminByte := avm.PopByteArray(engine)
	admin, err := common.Uint160ParseFromBytes(adminByte)
	if err != nil {
		return false, err
	}
	issueByte := avm.PopByteArray(engine)
	issue, err := common.Uint160ParseFromBytes(issueByte)
	if err != nil {
		return false, err
	}

	assetState := &states.AssetState{
		AssetId:    assetId,
		AssetType:  asset.AssetType(assertType),
		Name:       string(name),
		Amount:     common.Fixed64(amount.Int64()),
		Precision:  byte(precision.Int64()),
		Admin:      admin,
		Issuer:     issue,
		Owner:      owner,
		Expiration: ledger.DefaultLedger.Store.GetHeight() + 1 + 2000000,
		IsFrozen:   false,
	}
	s.CloneCache.GetInnerCache().GetWriteSet().Add(store.ST_AssetState, string(assetId.ToArray()), assetState)
	avm.PushData(engine, assetState)
	return true, nil
}

func (s *StateMachine) CreateContract(engine *avm.ExecutionEngine) (bool, error) {
	codeByte := avm.PopByteArray(engine)
	if len(codeByte) > 1024*1024 {
		return false, nil
	}
	parameters := avm.PopByteArray(engine)
	if len(parameters) > 252 {
		return false, nil
	}
	parameterList := make([]contract.ContractParameterType, 0)
	for _, v := range parameters {
		parameterList = append(parameterList, contract.ContractParameterType(v))
	}
	returnType := avm.PopInt(engine)
	nameByte := avm.PopByteArray(engine)
	if len(nameByte) > 252 {
		return false, nil
	}
	versionByte := avm.PopByteArray(engine)
	if len(versionByte) > 252 {
		return false, nil
	}
	authorByte := avm.PopByteArray(engine)
	if len(authorByte) > 252 {
		return false, nil
	}
	emailByte := avm.PopByteArray(engine)
	if len(emailByte) > 252 {
		return false, nil
	}
	descByte := avm.PopByteArray(engine)
	if len(emailByte) > 65536 {
		return false, nil
	}
	funcCode := &code.FunctionCode{
		Code:           codeByte,
		ParameterTypes: parameterList,
		ReturnType:     contract.ContractParameterType(returnType),
	}
	contractState := &states.ContractState{
		Code:        funcCode,
		Name:        hex.EncodeToString(nameByte),
		Version:     hex.EncodeToString(versionByte),
		Author:      hex.EncodeToString(authorByte),
		Email:       hex.EncodeToString(emailByte),
		Description: hex.EncodeToString(descByte),
	}
	codeHash, err := common.Uint160ParseFromBytes(codeByte)
	if err != nil {
		return false, err
	}
	s.CloneCache.GetInnerCache().GetOrAdd(store.ST_Contract, string(codeHash.ToArray()), contractState)
	avm.PushData(engine, contractState)
	return true, nil
}

func (s *StateMachine) GetContract(engine *avm.ExecutionEngine) (bool, error) {
	hashByte := avm.PopByteArray(engine)
	hash, err := common.Uint160ParseFromBytes(hashByte)
	if err != nil {
		return false, err
	}
	item, err := s.CloneCache.TryGet(store.ST_Contract, storage.KeyToStr(&hash))
	if err != nil {
		return false, err
	}
	avm.PushData(engine, item.(*states.ContractState))
	return true, nil
}

func (s *StateMachine) AssetRenew(engine *avm.ExecutionEngine) (bool, error) {
	data := avm.PopInteropInterface(engine)
	years := avm.PopInt(engine)
	at := data.(*states.AssetState)
	height := ledger.DefaultLedger.Store.GetHeight() + 1
	b := new(bytes.Buffer)
	at.AssetId.Serialize(b)
	state, err := s.CloneCache.TryGet(store.ST_AssetState, b.String())
	if err != nil {
		return false, err
	}
	assetState := state.(*states.AssetState)
	if assetState.Expiration < height {
		assetState.Expiration = height
	}
	assetState.Expiration += uint32(years) * 2000000
	return true, nil
}

func (s *StateMachine) ContractDestory(engine *avm.ExecutionEngine) (bool, error) {
	data := engine.CurrentContext().CodeHash
	if data != nil {
		return false, nil
	}
	hash, err := common.Uint160ParseFromBytes(data)
	if err != nil {
		return false, err
	}
	keyStr := storage.KeyToStr(&hash)
	item, err := s.CloneCache.TryGet(store.ST_Contract, keyStr)
	if err != nil || item == nil {
		return false, err
	}
	s.CloneCache.GetInnerCache().GetWriteSet().Delete(keyStr)
	return true, nil
}

func (s *StateMachine) CheckStorageContext(context *StorageContext) (bool, error) {
	item, err := s.CloneCache.TryGet(store.ST_Contract, string(context.codeHash.ToArray()))
	if err != nil {
		return false, err
	}
	if item == nil {
		return false, fmt.Errorf("check storage context fail, codehash=%v", context.codeHash)
	}
	return true, nil
}

func (s *StateMachine) StorageGet(engine *avm.ExecutionEngine) (bool, error) {
	opInterface := avm.PopInteropInterface(engine)
	context := opInterface.(*StorageContext)
	if exist, err := s.CheckStorageContext(context); !exist {
		return false, err
	}
	key := avm.PopByteArray(engine)
	storageKey := states.NewStorageKey(context.codeHash, key)
	item, err := s.CloneCache.TryGet(store.ST_Storage, storage.KeyToStr(storageKey))
	if err != nil && err.Error() != errors.NewErr("leveldb: not found").Error() {
		return false, err
	}
	if item == nil {
		avm.PushData(engine, []byte{})
	} else {
		avm.PushData(engine, item.(*states.StorageItem).Value)
	}
	return true, nil
}

func (s *StateMachine) StoragePut(engine *avm.ExecutionEngine) (bool, error) {
	opInterface := avm.PopInteropInterface(engine)
	context := opInterface.(*StorageContext)
	key := avm.PopByteArray(engine)
	value := avm.PopByteArray(engine)
	storageKey := states.NewStorageKey(context.codeHash, key)
	s.CloneCache.GetInnerCache().GetWriteSet().Add(store.ST_Storage, storage.KeyToStr(storageKey), states.NewStorageItem(value))
	return true, nil
}

func (s *StateMachine) StorageDelete(engine *avm.ExecutionEngine) (bool, error) {
	opInterface := avm.PopInteropInterface(engine)
	context := opInterface.(*StorageContext)
	key := avm.PopByteArray(engine)
	storageKey := states.NewStorageKey(context.codeHash, key)
	s.CloneCache.GetInnerCache().GetWriteSet().Delete(storage.KeyToStr(storageKey))
	return true, nil
}

func (s *StateMachine) GetStorageContext(engine *avm.ExecutionEngine) (bool, error) {
	return true, nil
}

func contains(programHashes []common.Uint160, programHash common.Uint160) bool {
	for _, v := range programHashes {
		if v == programHash {
			return true
		}
	}
	return false
}
