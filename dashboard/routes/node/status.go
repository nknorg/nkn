package node

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/dashboard/helpers"
	"github.com/nknorg/nkn/node"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"net/http"
)

func StatusRouter(router *gin.RouterGroup) {
	router.GET("/node/status", func(context *gin.Context) {
		var out map[string]interface{}

		localNode, exists := context.Get("localNode")

		if exists {
			buf, err := localNode.(*node.LocalNode).MarshalJSON()
			if err != nil {
				log.WebLog.Error(err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			err = json.Unmarshal(buf, &out)
			if err != nil {
				log.WebLog.Error(err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			out["beneficiaryAddr"] = config.Parameters.BeneficiaryAddr

			data := helpers.EncryptData(context, true, out)

			context.JSON(http.StatusOK, gin.H{
				"data": data,
			})
			return
		}

	})
}
