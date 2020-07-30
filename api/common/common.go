package common

import (
	"crypto/tls"

	"github.com/nknorg/nkn/v2/node"
	"github.com/nknorg/nkn/v2/util/config"
	"github.com/nknorg/nkn/v2/util/log"
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

func GetHttpsCertificate(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(config.Parameters.HttpsJsonCert, config.Parameters.HttpsJsonKey)
	if err != nil {
		log.Error("load https json keys fail", err)
		return nil, err
	}
	return &cert, nil
}

func GetWssCertificate(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(config.Parameters.HttpWssCert, config.Parameters.HttpWssKey)
	if err != nil {
		log.Error("load https wss keys fail", err)
		return nil, err
	}
	return &cert, nil
}
