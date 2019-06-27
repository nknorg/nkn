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

	ST_Prepaid   DataEntryPrefix = 0xc7
	ST_StateTrie DataEntryPrefix = 0xc8

	//SYSTEM
	SYS_CurrentBlock DataEntryPrefix = 0x40
	SYS_Donations    DataEntryPrefix = 0x42

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

func donationKey(height uint32) []byte {
	heightBuffer := make([]byte, 4)
	binary.LittleEndian.PutUint32(heightBuffer[:], height)
	return paddingKey(SYS_Donations, heightBuffer)
}

func currentStateTrie() []byte {
	return paddingKey(ST_StateTrie, nil)
}

func prepaidKey(programHash common.Uint160) []byte {
	return paddingKey(ST_Prepaid, programHash.ToArray())
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
