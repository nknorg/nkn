package node

import (
	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/dashboard/helpers"
	"github.com/nknorg/nkn/node"
	"net/http"
)

func NeighborRouter(router *gin.RouterGroup) {
	router.GET("/node/neighbors", func(context *gin.Context) {
		localNode, exists := context.Get("localNode")

		if exists {
			list := localNode.(*node.LocalNode).GetNeighborInfo()

			data := helpers.EncryptData(context, true, list)
			context.JSON(http.StatusOK, gin.H{
				"data": data,
			})
			return
		}

	})
}
