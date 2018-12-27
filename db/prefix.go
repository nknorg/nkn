package db

type DataEntryPrefix byte

const (
	// DATA
	DATA_BlockHash   DataEntryPrefix = 0x00
	DATA_Header      DataEntryPrefix = 0x01
	DATA_Transaction DataEntryPrefix = 0x02

	// INDEX
	IX_HeaderHashList DataEntryPrefix = 0x80
	IX_Unspent        DataEntryPrefix = 0x90
	IX_Unspent_UTXO   DataEntryPrefix = 0x91

	// ASSET
	ST_Info           DataEntryPrefix = 0xc0
	ST_QuantityIssued DataEntryPrefix = 0xc1
	ST_Contract       DataEntryPrefix = 0xc2
	ST_Storage        DataEntryPrefix = 0xc3
	ST_Prepaid        DataEntryPrefix = 0xc7

	//SYSTEM
	SYS_CurrentBlock DataEntryPrefix = 0x40
	SYS_ExpireKey    DataEntryPrefix = 0x41

	// NAME
	NS_Registrant DataEntryPrefix = 0x50
	NS_Name       DataEntryPrefix = 0x51

	// PUBSUB
	PS_Topic DataEntryPrefix = 0x60

	//CONFIG
	CFG_Version DataEntryPrefix = 0xf0
)
