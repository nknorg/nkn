package db

import (
	"encoding/binary"

	"github.com/nknorg/nkn/v2/common"
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

	//TRIE
	TRIE_Node               DataEntryPrefix = 0xa0
	TRIE_RefCount           DataEntryPrefix = 0xa1
	TRIE_RefCountHeight     DataEntryPrefix = 0xa2
	TRIE_PrunedHeight       DataEntryPrefix = 0xa3
	TRIE_CompactHeight      DataEntryPrefix = 0xa4
	TRIE_FastSyncStatus     DataEntryPrefix = 0xa5
	TRIE_FastSyncRoot       DataEntryPrefix = 0xa6
	TRIE_FastSyncRootHeight DataEntryPrefix = 0xa7
	TRIE_RefCountNeedReset  DataEntryPrefix = 0xa8
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

func CurrentFastSyncRoot() []byte {
	return paddingKey(TRIE_FastSyncRoot, nil)
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

func TrieRefCountKey(key []byte) []byte {
	return paddingKey(TRIE_RefCount, key)
}

func TrieRefCountHeightKey() []byte {
	return paddingKey(TRIE_RefCountHeight, nil)
}

func TrieRefCountNeedResetKey() []byte {
	return paddingKey(TRIE_RefCountNeedReset, nil)
}

func TriePrunedHeightKey() []byte {
	return paddingKey(TRIE_PrunedHeight, nil)
}

func TrieCompactHeightKey() []byte {
	return paddingKey(TRIE_CompactHeight, nil)
}

func TrieFastSyncRootHeightKey() []byte {
	return paddingKey(TRIE_FastSyncRootHeight, nil)
}

func TrieFastSyncStatusKey() []byte {
	return paddingKey(TRIE_FastSyncStatus, nil)
}
