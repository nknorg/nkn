package states

import (
	"io"
	"nkn-core/vm/avm/interfaces"
	"nkn-core/core/store"
	"bytes"
)

type IStateValueInterface interface {
	Serialize(w io.Writer) error
	Deserialize(r io.Reader) error
	interfaces.IInteropInterface
}

type IStateKeyInterface interface {
	Serialize(w io.Writer) (int, error)
	Deserialize(r io.Reader) error
}

var (
	StatesMap = map[store.DataEntryPrefix]IStateValueInterface{
		store.ST_Contract: new(ContractState),
		store.ST_Storage: new(StorageItem),
		store.ST_ACCOUNT: new(AccountState),
		store.ST_AssetState: new(AssetState),
		store.ST_Validator: new(ValidatorState),
	}
)

func GetStateValue(prefix store.DataEntryPrefix, data []byte) (IStateValueInterface, error){
	r := bytes.NewBuffer(data)
	state := StatesMap[prefix]
	if err := state.Deserialize(r); err != nil {
		return nil, err
	}
	return state, nil
}
