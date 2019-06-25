package node

import (
	"github.com/gin-gonic/gin"
)

type NodeRouter struct {
}

func (nodeRouter *NodeRouter) Router(router *gin.RouterGroup) {
	router.GET("/node/status", func(context *gin.Context) {
		node, exists := context.Get("localNode")
		if exists {
			context.JSON(200, node)
			return
		}

		context.JSON(200, gin.H{
			"status": "...",
		})
	})
}
