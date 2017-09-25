package states

import (
	"testing"
	"DNA/common"
	"bytes"
	"fmt"
	"DNA/crypto"
	"math/big"
	"DNA/common/serialization"
)

func TestAccountState(t *testing.T) {
	b := new(bytes.Buffer)
	m := make(map[common.Uint256]common.Fixed64, 0)
	k1 := [32]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	k2 := [32]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}

	m[k1] = common.Fixed64(1)
	m[k2] = common.Fixed64(2)
	accountState := &AccountState{
		ProgramHash: [20]uint8{0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1},
		IsFrozen: true,
		Balances: m,
	}
	accountState.Serialize(b)
	accountState = &AccountState{}
	fmt.Printf("serialize accountstate:%+v\n", b.Bytes())

	accountState.Deserialize(b)
	fmt.Printf("deserialize accountstate:%+v\n", accountState)
}

func TestAssetState(t *testing.T) {
	b := new(bytes.Buffer)
	pubkey := &crypto.PubKey{X: big.NewInt(1), Y: big.NewInt(2)}
	assetState := &AssetState{
		AssetId: [32]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		Name: "AssetStateTest",
		Amount: common.Fixed64(1),
		Available: common.Fixed64(1),
		Precision: byte(1),
		FeeMode: byte(1),
		Fee: common.Fixed64(1),
		FeeAddress: [20]uint8{0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1},
		Owner: pubkey,
		Admin: [20]uint8{0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,2},
		Issuer: [20]uint8{0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,3},
		Expiration: uint64(123456789123),
		IsFrozen: true,
	}
	assetState.Serialize(b)
	fmt.Printf("serialize assetstate:%+v\n", b.Bytes())
	assetState = &AssetState{}
	assetState.Deserialize(b)
	fmt.Printf("deserialize assetstate:%+v\n", assetState)

}

func TestContractState(t *testing.T) {
	b := new(bytes.Buffer)
	a8 := []byte{10, 11, 12}
	serialization.WriteVarBytes(b, a8)
	fmt.Println(serialization.ReadVarBytes(b))

	code := []byte{10, 11, 12}
	contractState := &ContractState{
		Code: code,
	}
	contractState.Serialize(b)
	fmt.Printf("serialize contractstate:%+v\n", b.Bytes())
	contractState = &ContractState{}
	contractState.Deserialize(b)
	fmt.Printf("deserialize assetstate:%+v\n", contractState)
}

func TestStorageItem(t *testing.T) {
	b := new(bytes.Buffer)
	storageItem := &StorageItem{
		Value: []byte{10, 11, 12},
	}
	storageItem.Serialize(b)
	fmt.Printf("serialize storageitem:%+v\n", b.Bytes())
	storageItem = &StorageItem{}
	storageItem.Deserialize(b)
	fmt.Printf("deserialize storageitem:%+v\n", storageItem)
}

func TestStorageKey(t *testing.T) {
	b := new(bytes.Buffer)
	storageKey := &StorageKey{
		CodeHash: [20]uint8{0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,2},
		Key: []byte{10, 11, 12},
	}
	storageKey.Serialize(b)
	fmt.Printf("serialize storagekey:%+v\n", b.Bytes())
	storageKey = &StorageKey{}
	storageKey.Deserialize(b)
	fmt.Printf("deserialize storagekey:%+v\n", storageKey)
}

