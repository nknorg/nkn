package common

import (
	"github.com/nknorg/nkn/node"
)

type Serverer interface {
	GetNetNode() *node.LocalNode
}

// Response for json API.
// errcode: The error code to return to client, see api/common/errcode.go
// reusltOrData: If the errcode is 0, then data is used as the 'result' of JsonRPC. Otherwise,
// as a extra error message to 'data' of JsonRPC.
func respPacking(errcode ErrCode, resultOrData interface{}) map[string]interface{} {
	resp := map[string]interface{}{
		"error":        errcode,
		"resultOrData": resultOrData,
	}
	return resp
}

func RespPacking(result interface{}, errcode ErrCode) map[string]interface{} {
	return respPacking(errcode, result)
}

func ResponsePack(errCode ErrCode) map[string]interface{} {
	resp := map[string]interface{}{
		"Action":  "",
		"Result":  "",
		"Error":   errCode,
		"Desc":    "",
		"Version": 1,
	}
	return resp
}
