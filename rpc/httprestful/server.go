package httprestful

import (
	"nkn-core/rpc/httprestful/common"
	. "nkn-core/rpc/httprestful/restful"
	. "nkn-core/net/protocol"
)

func StartServer(n Noder) {
	common.SetNode(n)
	func() {
		rest := InitRestServer()
		go rest.Start()
	}()
}