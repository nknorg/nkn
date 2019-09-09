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

	//TRIE NODE
	TRIE_Node DataEntryPrefix = 0xa0
)

func paddingKey(prefix DataEntryPrefix, key []byte) []byte {
	return append([]byte{byte(prefix)}, key...)
}

func VersionKey() []byte {
	return paddingKey(CFG_Version, nil)
}

func CurrentBlockHashKey() []byte {
	return paddingKey(SYS_CurrentBlock, nil)
}

func DonationKey(height uint32) []byte {
	heightBuffer := make([]byte, 4)
	binary.LittleEndian.PutUint32(heightBuffer[:], height)
	return paddingKey(SYS_Donations, heightBuffer)
}

func CurrentStateTrie() []byte {
	return paddingKey(ST_StateTrie, nil)
}

func PrepaidKey(programHash common.Uint160) []byte {
	return paddingKey(ST_Prepaid, programHash.ToArray())
}

func BlockhashKey(height uint32) []byte {
	heightBuffer := make([]byte, 4)
	binary.LittleEndian.PutUint32(heightBuffer[:], height)

	return paddingKey(DATA_BlockHash, heightBuffer)
}

func HeaderKey(blockHash common.Uint256) []byte {
	return paddingKey(DATA_Header, blockHash.ToArray())
}

func TransactionKey(txHash common.Uint256) []byte {
	return paddingKey(DATA_Transaction, txHash.ToArray())
}

func TrieNodeKey(key []byte) []byte {
	return paddingKey(TRIE_Node, key)
}
