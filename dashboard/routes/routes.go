package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/webservice/routes/node"
	"github.com/nknorg/nkn/webservice/routes/wallet"
)

func Routes(app *gin.Engine) gin.HandlerFunc {
	nodeRouter := &node.NodeRouter{}
	nodeRouter.Router(app.Group("/api"))

	walletRouter := &wallet.WalletRouter{}
	walletRouter.Router(app.Group("/api"))

	return func(c *gin.Context) {

	}
}
