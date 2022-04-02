package node

import (
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/dashboard/helpers"
	"github.com/nknorg/nkn/v2/node"
	"github.com/nknorg/nkn/v2/util/log"
)

func StatusRouter(router *gin.RouterGroup) {
	router.GET("/node/status", func(context *gin.Context) {
		var out map[string]interface{} = make(map[string]interface{})

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
			id := context.MustGet("id").([]byte)
			out["id"] = hex.EncodeToString(id)
		}

		out["beneficiaryAddr"] = config.Parameters.BeneficiaryAddr
		out["registerIDTxnFee"] = config.Parameters.RegisterIDTxnFee
		out["numLowFeeTxnPerBlock"] = config.Parameters.NumLowFeeTxnPerBlock
		out["lowFeeTxnSizePerBlock"] = config.Parameters.LowFeeTxnSizePerBlock
		out["lowTxnFee"] = config.Parameters.LowTxnFee
		out["lowTxnFeePerSize"] = config.Parameters.LowTxnFeePerSize

		data := helpers.EncryptData(context, true, out)

		context.JSON(http.StatusOK, gin.H{
			"data": data,
		})
		return

	})
}
