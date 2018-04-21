package httprestful

import (
	"github.com/nknorg/nkn/rpc/httprestful/common"
	. "github.com/nknorg/nkn/rpc/httprestful/restful"
	. "github.com/nknorg/nkn/net/protocol"
)

func StartServer(n Noder) {
	common.SetNode(n)
	func() {
		rest := InitRestServer()
		go rest.Start()
	}()
}
