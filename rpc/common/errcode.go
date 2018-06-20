package common

type ErrCode int64

const (
	SUCCESS            ErrCode = 0
	SESSION_EXPIRED    ErrCode = 41001
	SERVICE_CEILING    ErrCode = 41002
	ILLEGAL_DATAFORMAT ErrCode = 41003

	INVALID_METHOD ErrCode = 42001
	INVALID_PARAMS ErrCode = 42002
	INVALID_TOKEN  ErrCode = 42003

	INVALID_TRANSACTION ErrCode = 43001
	INVALID_ASSET       ErrCode = 43002
	INVALID_BLOCK       ErrCode = 43003

	UNKNOWN_TRANSACTION ErrCode = 44001
	UNKNOWN_ASSET       ErrCode = 44002
	UNKNOWN_BLOCK       ErrCode = 44003

	INVALID_VERSION ErrCode = 45001
	INTERNAL_ERROR  ErrCode = 45002

	SMARTCODE_ERROR ErrCode = 47001
)

var ErrMap = map[ErrCode]string{
	SUCCESS:            "SUCCESS",
	SESSION_EXPIRED:    "SESSION EXPIRED",
	SERVICE_CEILING:    "SERVICE CEILING",
	ILLEGAL_DATAFORMAT: "ILLEGAL DATAFORMAT",

	INVALID_METHOD: "INVALID METHOD",
	INVALID_PARAMS: "INVALID PARAMS",
	INVALID_TOKEN:  "VERIFY TOKEN ERROR",

	INVALID_TRANSACTION: "INVALID TRANSACTION",
	INVALID_ASSET:       "INVALID ASSET",
	INVALID_BLOCK:       "INVALID BLOCK",

	UNKNOWN_TRANSACTION: "UNKNOWN TRANSACTION",
	UNKNOWN_ASSET:       "UNKNOWN ASSET",
	UNKNOWN_BLOCK:       "UNKNOWN BLOCK",

	INVALID_VERSION: "INVALID VERSION",
	INTERNAL_ERROR:  "INTERNAL ERROR",
	SMARTCODE_ERROR: "SMARTCODE EXEC ERROR",
	//ErrCode(ErrDuplicateInput):       "INTERNAL ERROR, ErrDuplicateInput",
	//ErrCode(ErrAssetPrecision):       "INTERNAL ERROR, ErrAssetPrecision",
	//ErrCode(ErrTransactionBalance):   "INTERNAL ERROR, ErrTransactionBalance",
	//ErrCode(ErrAttributeProgram):     "INTERNAL ERROR, ErrAttributeProgram",
	//ErrCode(ErrTransactionContracts): "INTERNAL ERROR, ErrTransactionContracts",
	//ErrCode(ErrTransactionPayload):   "INTERNAL ERROR, ErrTransactionPayload",
	//ErrCode(ErrDoubleSpend):          "INTERNAL ERROR, ErrDoubleSpend",
	//ErrCode(ErrTxHashDuplicate):      "INTERNAL ERROR, ErrTxHashDuplicate",
	//ErrCode(ErrStateUpdaterVaild):    "INTERNAL ERROR, ErrStateUpdaterVaild",
	//ErrCode(ErrSummaryAsset):         "INTERNAL ERROR, ErrSummaryAsset",
	//ErrCode(ErrXmitFail):             "INTERNAL ERROR, ErrXmitFail",
}
