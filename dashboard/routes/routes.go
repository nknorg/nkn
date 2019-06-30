package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/dashboard/routes/node"
	"github.com/nknorg/nkn/dashboard/routes/wallet"
)

func Routes(app *gin.Engine) gin.HandlerFunc {
	node.StatusRouter(app.Group("/api"))
	node.BeneficiaryRouter(app.Group("/api"))
	node.NeighborRouter(app.Group("/api"))

	wallet.StatusRouter(app.Group("/api"))

	return func(c *gin.Context) {

	}
}
