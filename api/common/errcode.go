package common

import "github.com/nknorg/nkn/errors"

type ErrCode int64

const (
	SUCCESS             ErrCode = 0
	SESSION_EXPIRED     ErrCode = 41001
	SERVICE_CEILING     ErrCode = 41002
	ILLEGAL_DATAFORMAT  ErrCode = 41003
	INVALID_METHOD      ErrCode = 42001
	INVALID_PARAMS      ErrCode = 42002
	INVALID_TOKEN       ErrCode = 42003
	INVALID_TRANSACTION ErrCode = 43001
	INVALID_ASSET       ErrCode = 43002
	INVALID_BLOCK       ErrCode = 43003
	INVALID_HASH        ErrCode = 43004
	INVALID_VERSION     ErrCode = 43005
	UNKNOWN_TRANSACTION ErrCode = 44001
	UNKNOWN_ASSET       ErrCode = 44002
	UNKNOWN_BLOCK       ErrCode = 44003
	UNKNOWN_HASH        ErrCode = 44004
	INTERNAL_ERROR      ErrCode = 45001
	SMARTCODE_ERROR     ErrCode = 47001
	WRONG_NODE          ErrCode = 48001
)

var ErrMessage = map[ErrCode]string{
	SUCCESS:                                 "SUCCESS",
	SESSION_EXPIRED:                         "SESSION EXPIRED",
	SERVICE_CEILING:                         "SERVICE CEILING",
	ILLEGAL_DATAFORMAT:                      "ILLEGAL DATAFORMAT",
	INVALID_METHOD:                          "INVALID METHOD",
	INVALID_PARAMS:                          "INVALID PARAMS",
	INVALID_TOKEN:                           "VERIFY TOKEN ERROR",
	INVALID_TRANSACTION:                     "INVALID TRANSACTION",
	INVALID_ASSET:                           "INVALID ASSET",
	INVALID_BLOCK:                           "INVALID BLOCK",
	INVALID_HASH:                            "INVALID HASH",
	INVALID_VERSION:                         "INVALID VERSION",
	UNKNOWN_TRANSACTION:                     "UNKNOWN TRANSACTION",
	UNKNOWN_ASSET:                           "UNKNOWN ASSET",
	UNKNOWN_BLOCK:                           "UNKNOWN BLOCK",
	UNKNOWN_HASH:                            "UNKNOWN HASH",
	INTERNAL_ERROR:                          "INTERNAL ERROR",
	SMARTCODE_ERROR:                         "SMARTCODE EXEC ERROR",
	WRONG_NODE:                              "WRONG NODE TO CONNECT",
	ErrCode(errors.ErrDuplicateInput):       "INTERNAL ERROR, ErrDuplicateInput",
	ErrCode(errors.ErrAssetPrecision):       "INTERNAL ERROR, ErrAssetPrecision",
	ErrCode(errors.ErrTransactionBalance):   "INTERNAL ERROR, ErrTransactionBalance",
	ErrCode(errors.ErrAttributeProgram):     "INTERNAL ERROR, ErrAttributeProgram",
	ErrCode(errors.ErrTransactionContracts): "INTERNAL ERROR, ErrTransactionContracts",
	ErrCode(errors.ErrTransactionPayload):   "INTERNAL ERROR, ErrTransactionPayload",
	ErrCode(errors.ErrDoubleSpend):          "INTERNAL ERROR, ErrDoubleSpend",
	ErrCode(errors.ErrTxHashDuplicate):      "INTERNAL ERROR, ErrTxHashDuplicate",
	ErrCode(errors.ErrStateUpdaterVaild):    "INTERNAL ERROR, ErrStateUpdaterVaild",
	ErrCode(errors.ErrSummaryAsset):         "INTERNAL ERROR, ErrSummaryAsset",
	ErrCode(errors.ErrXmitFail):             "INTERNAL ERROR, ErrXmitFail",
}
