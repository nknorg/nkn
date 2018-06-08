package httprestful

import (
	. "github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/rpc/httprestful/common"
	. "github.com/nknorg/nkn/rpc/httprestful/restful"
)

func StartServer(n Noder) {
	common.SetNode(n)
	func() {
		rest := InitRestServer()
		go rest.Start()
	}()
}
