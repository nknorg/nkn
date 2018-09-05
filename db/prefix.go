package db

import (
	"encoding/binary"

	"github.com/nknorg/nkn/common"
)

type DataEntryPrefix byte

const (
	// DATA
	DATA_BlockHash   DataEntryPrefix = 0x00
	DATA_Header      DataEntryPrefix = 0x01
	DATA_Transaction DataEntryPrefix = 0x02

	// INDEX
	IX_Unspent      DataEntryPrefix = 0x90
	IX_Unspent_UTXO DataEntryPrefix = 0x91

	// ASSET
	ST_Info           DataEntryPrefix = 0xc0
	ST_QuantityIssued DataEntryPrefix = 0xc1
	ST_Prepaid        DataEntryPrefix = 0xc7

	//SYSTEM
	SYS_CurrentBlock      DataEntryPrefix = 0x40
	SYS_CurrentBookKeeper DataEntryPrefix = 0x42

	//CONFIG
	CFG_Version DataEntryPrefix = 0xf0
)

func paddingKey(prefix DataEntryPrefix, key []byte) []byte {
	return append([]byte{byte(prefix)}, key...)
}

func versionKey() []byte {
	return paddingKey(CFG_Version, nil)
}

func currentBlockHashKey() []byte {
	return paddingKey(SYS_CurrentBlock, nil)
}

func bookkeeperKey() []byte {
	return paddingKey(SYS_CurrentBookKeeper, nil)
}

func assetKey(assetId common.Uint256) []byte {
	return paddingKey(ST_Info, assetId.ToArray())
}

func issuseQuantKey(assetId common.Uint256) []byte {
	return paddingKey(ST_QuantityIssued, assetId.ToArray())
}

func prepaidKey(programHash common.Uint160) []byte {
	return paddingKey(ST_Prepaid, programHash.ToArray())
}

func unspentIndexKey(txHash common.Uint256) []byte {
	return paddingKey(IX_Unspent, txHash.ToArray())
}

func unspentUtxoKey(programHash common.Uint160, assetid common.Uint256, height uint32) []byte {
	heightBuffer := make([]byte, 4)
	binary.LittleEndian.PutUint32(heightBuffer[:], height)
	key := append(append(programHash.ToArray(), assetid.ToArray()...), heightBuffer...)

	return paddingKey(IX_Unspent_UTXO, key)
}

func blockhashKey(height uint32) []byte {
	heightBuffer := make([]byte, 4)
	binary.LittleEndian.PutUint32(heightBuffer[:], height)

	return paddingKey(DATA_BlockHash, heightBuffer)
}

func headerKey(blockHash common.Uint256) []byte {
	return paddingKey(DATA_Header, blockHash.ToArray())
}

func transactionKey(txHash common.Uint256) []byte {
	return paddingKey(DATA_Transaction, txHash.ToArray())
}

func iteratorBlockHash(store IStore) IIterator {
	return store.NewIterator([]byte{byte(DATA_BlockHash)})
}

func iteratorAsset(store IStore) IIterator {
	return store.NewIterator([]byte{byte(ST_Info)})
}

func iteratorUTXO(store IStore, key []byte) IIterator {
	return store.NewIterator(append([]byte{byte(IX_Unspent_UTXO)}, key...))

}
