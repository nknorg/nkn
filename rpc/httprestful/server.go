package httprestful

import (
	"nkn/rpc/httprestful/common"
	. "nkn/rpc/httprestful/restful"
	. "nkn/net/protocol"
)

func StartServer(n Noder) {
	common.SetNode(n)
	func() {
		rest := InitRestServer()
		go rest.Start()
	}()
}
