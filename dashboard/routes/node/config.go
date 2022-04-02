package node

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/dashboard/auth"
	"github.com/nknorg/nkn/v2/dashboard/helpers"
	"github.com/nknorg/nkn/v2/util/log"
)

type SetConfigData struct {
	RegisterIDTxnFee      int64   `form:"registerIDTxnFee" binding:"required"`
	NumLowFeeTxnPerBlock  uint32  `form:"numLowFeeTxnPerBlock" binding:"required"`
	LowFeeTxnSizePerBlock uint32  `form:"lowFeeTxnSizePerBlock" binding:"required"`
	LowTxnFee             int64   `form:"lowTxnFee" binding:"required"`
	LowTxnFeePerSize      float64 `form:"lowTxnFeePerSize" binding:"required"`
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
		configuration["LowTxnFee"] = data.LowTxnFee
		configuration["LowTxnFeePerSize"] = data.LowTxnFeePerSize
		err = config.WriteConfigFile(configuration)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		config.Parameters.RegisterIDTxnFee = data.RegisterIDTxnFee
		config.Parameters.NumLowFeeTxnPerBlock = data.NumLowFeeTxnPerBlock
		config.Parameters.LowFeeTxnSizePerBlock = data.LowFeeTxnSizePerBlock
		config.Parameters.LowTxnFee = data.LowTxnFee
		config.Parameters.LowTxnFeePerSize = data.LowTxnFeePerSize

		context.JSON(http.StatusOK, "")
		return

	})

}
