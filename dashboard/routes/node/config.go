package node

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/dashboard/auth"
	"github.com/nknorg/nkn/dashboard/helpers"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"net/http"
)

type SetConfigData struct {
	RegisterIDTxnFee      int64  `form:"registerIDTxnFee" binding:"required"`
	NumLowFeeTxnPerBlock  uint32 `form:"numLowFeeTxnPerBlock" binding:"required"`
	LowFeeTxnSizePerBlock uint32 `form:"lowFeeTxnSizePerBlock" binding:"required"`
	MinTxnFee             int64  `form:"minTxnFee" binding:"required"`
}

func NodeConfigRouter(router *gin.RouterGroup) {
	router.PUT("/node/config", auth.WalletAuth(), func(context *gin.Context) {
		bodyData := helpers.DecryptData(context, true)
		var data SetConfigData
		err := json.Unmarshal([]byte(bodyData), &data)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusBadRequest, err)
			return
		}

		file, err := config.OpenConfigFile()
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		var configuration map[string]interface{}
		err = json.Unmarshal(file, &configuration)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		// set config
		configuration["RegisterIDTxnFee"] = data.RegisterIDTxnFee
		configuration["NumLowFeeTxnPerBlock"] = data.NumLowFeeTxnPerBlock
		configuration["LowFeeTxnSizePerBlock"] = data.LowFeeTxnSizePerBlock
		configuration["MinTxnFee"] = data.MinTxnFee
		err = config.WriteConfigFile(configuration)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		config.Parameters.RegisterIDTxnFee = data.RegisterIDTxnFee
		config.Parameters.NumLowFeeTxnPerBlock = data.NumLowFeeTxnPerBlock
		config.Parameters.LowFeeTxnSizePerBlock = data.LowFeeTxnSizePerBlock
		config.Parameters.MinTxnFee = data.MinTxnFee

		context.JSON(http.StatusOK, "")
		return

	})

}
