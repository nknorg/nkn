package httprestful

import (
	"nkn-core/net/httprestful/common"
	. "nkn-core/net/httprestful/restful"
	. "nkn-core/net/protocol"
)

func StartServer(n Noder) {
	common.SetNode(n)
	func() {
		rest := InitRestServer()
		go rest.Start()
	}()
}