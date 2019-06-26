package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/dashboard/routes/node"
	"github.com/nknorg/nkn/dashboard/routes/wallet"
)

func Routes(app *gin.Engine) gin.HandlerFunc {
	nodeRouter := &node.NodeRouter{}
	nodeRouter.Router(app.Group("/api"))

	walletRouter := &wallet.WalletRouter{}
	walletRouter.Router(app.Group("/api"))

	return func(c *gin.Context) {

	}
}
