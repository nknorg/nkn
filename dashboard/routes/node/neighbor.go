package node

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/v2/dashboard/helpers"
	"github.com/nknorg/nkn/v2/lnode"
)

func NeighborRouter(router *gin.RouterGroup) {
	router.GET("/node/neighbors", func(context *gin.Context) {
		localNode, exists := context.Get("localNode")

		if exists {
			list := localNode.(*lnode.LocalNode).GetNeighborInfo()

			data := helpers.EncryptData(context, true, list)
			context.JSON(http.StatusOK, gin.H{
				"data": data,
			})
			return
		}

	})
}
