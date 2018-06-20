package httpjson

var (
	RpcResultInvalidHash        = responsePacking("invalid hash")
	RpcResultInvalidBlock       = responsePacking("invalid block")
	RpcResultInvalidTransaction = responsePacking("invalid transaction")
	RpcResultInvalidParameter   = responsePacking("invalid parameter")

	RpcResultUnknownBlock       = responsePacking("unknown block")
	RpcResultUnknownTransaction = responsePacking("unknown transaction")

	RpcResultNil           = responsePacking(nil)
	RpcResultUnsupported   = responsePacking("Unsupported")
	RpcResultInternalError = responsePacking("internal error")
	RpcResultIOError       = responsePacking("internal IO error")
	RpcResultAPIError      = responsePacking("internal API error")
	RpcResultSuccess       = responsePacking(true)
	RpcResultFailed        = responsePacking(false)

	RpcResult = responsePacking
)
