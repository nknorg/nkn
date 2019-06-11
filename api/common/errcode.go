package common

type ErrCode int64

const (
	SUCCESS                  ErrCode = 0
	SESSION_EXPIRED          ErrCode = 41001
	SERVICE_CEILING          ErrCode = 41002
	ILLEGAL_DATAFORMAT       ErrCode = 41003
	INVALID_METHOD           ErrCode = 42001
	INVALID_PARAMS           ErrCode = 42002
	INVALID_TOKEN            ErrCode = 42003
	INVALID_TRANSACTION      ErrCode = 43001
	INVALID_ASSET            ErrCode = 43002
	INVALID_BLOCK            ErrCode = 43003
	INVALID_HASH             ErrCode = 43004
	INVALID_VERSION          ErrCode = 43005
	UNKNOWN_TRANSACTION      ErrCode = 44001
	UNKNOWN_ASSET            ErrCode = 44002
	UNKNOWN_BLOCK            ErrCode = 44003
	UNKNOWN_HASH             ErrCode = 44004
	INTERNAL_ERROR           ErrCode = 45001
	SMARTCODE_ERROR          ErrCode = 47001
	WRONG_NODE               ErrCode = 48001
	ErrNoCode                ErrCode = -2
	ErrNoError               ErrCode = 0
	ErrUnknown               ErrCode = -1
	ErrDuplicatedTx          ErrCode = 1
	ErrInputOutputTooLong    ErrCode = 45002
	ErrDuplicateInput        ErrCode = 45003
	ErrAssetPrecision        ErrCode = 45004
	ErrTransactionBalance    ErrCode = 45005
	ErrAttributeProgram      ErrCode = 45006
	ErrTransactionContracts  ErrCode = 45007
	ErrTransactionPayload    ErrCode = 45008
	ErrDoubleSpend           ErrCode = 45009
	ErrTxHashDuplicate       ErrCode = 45010
	ErrStateUpdaterVaild     ErrCode = 45011
	ErrSummaryAsset          ErrCode = 45012
	ErrXmitFail              ErrCode = 45013
	ErrDuplicateName         ErrCode = 45015
	ErrMineReward            ErrCode = 45016
	ErrDuplicateSubscription ErrCode = 45017
	ErrSubscriptionLimit     ErrCode = 45018
	ErrDoNotPropagate        ErrCode = 45019
	ErrAlreadySubscribed     ErrCode = 45020
	ErrAppendTxnPool         ErrCode = 45021
	ErrNullID                ErrCode = 45022
	ErrZeroID                ErrCode = 45023
)

var ErrMessage = map[ErrCode]string{
	SUCCESS:                 "SUCCESS",
	SESSION_EXPIRED:         "SESSION EXPIRED",
	SERVICE_CEILING:         "SERVICE CEILING",
	ILLEGAL_DATAFORMAT:      "ILLEGAL DATAFORMAT",
	INVALID_METHOD:          "INVALID METHOD",
	INVALID_PARAMS:          "INVALID PARAMS",
	INVALID_TOKEN:           "VERIFY TOKEN ERROR",
	INVALID_TRANSACTION:     "INVALID TRANSACTION",
	INVALID_ASSET:           "INVALID ASSET",
	INVALID_BLOCK:           "INVALID BLOCK",
	INVALID_HASH:            "INVALID HASH",
	INVALID_VERSION:         "INVALID VERSION",
	UNKNOWN_TRANSACTION:     "UNKNOWN TRANSACTION",
	UNKNOWN_ASSET:           "UNKNOWN ASSET",
	UNKNOWN_BLOCK:           "UNKNOWN BLOCK",
	UNKNOWN_HASH:            "UNKNOWN HASH",
	INTERNAL_ERROR:          "INTERNAL ERROR",
	SMARTCODE_ERROR:         "SMARTCODE EXEC ERROR",
	WRONG_NODE:              "WRONG NODE TO CONNECT",
	ErrDuplicatedTx:         "INTERNAL ERROR, Duplicate transaction",
	ErrDuplicateInput:       "INTERNAL ERROR, ErrDuplicateInput",
	ErrAssetPrecision:       "INTERNAL ERROR, ErrAssetPrecision",
	ErrTransactionBalance:   "INTERNAL ERROR, ErrTransactionBalance",
	ErrAttributeProgram:     "INTERNAL ERROR, ErrAttributeProgram",
	ErrTransactionContracts: "INTERNAL ERROR, ErrTransactionContracts",
	ErrTransactionPayload:   "INTERNAL ERROR, ErrTransactionPayload",
	ErrDoubleSpend:          "INTERNAL ERROR, ErrDoubleSpend",
	ErrTxHashDuplicate:      "INTERNAL ERROR, ErrTxHashDuplicate",
	ErrStateUpdaterVaild:    "INTERNAL ERROR, ErrStateUpdaterVaild",
	ErrSummaryAsset:         "INTERNAL ERROR, ErrSummaryAsset",
	ErrXmitFail:             "INTERNAL ERROR, ErrXmitFail",
	ErrAppendTxnPool:        "INTERNAL ERROR, can not append tx to txpool",
	ErrNullID:               "INTERNAL ERROR, there is no ID in account",
	ErrZeroID:               "INTERNAL ERROR, it's zero ID in account",
}
