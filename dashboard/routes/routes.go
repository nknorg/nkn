package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/dashboard/auth"
	"github.com/nknorg/nkn/dashboard/routes/common"
	"github.com/nknorg/nkn/dashboard/routes/node"
	"github.com/nknorg/nkn/dashboard/routes/wallet"
)

func Routes(app *gin.Engine) gin.HandlerFunc {
	common.SyncRouter(app.Group("/api"))

	app.Use(auth.UnixRangeAuth())

	node.StatusRouter(app.Group("/api"))
	node.BeneficiaryRouter(app.Group("/api"))
	node.NeighborRouter(app.Group("/api"))

	wallet.StatusRouter(app.Group("/api"))
	wallet.WalletCreateRouter(app.Group("/api"))
	wallet.WalletOpenRouter(app.Group("/api"))
	wallet.WalletDownloadRouter(app.Group("/api"))

	return func(c *gin.Context) {

	}
}
