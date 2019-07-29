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

type SetBeneficiaryData struct {
	BeneficiaryAddr string `form:"beneficiaryAddr" binding:"required"`
}

func BeneficiaryRouter(router *gin.RouterGroup) {
	router.PUT("/node/beneficiary", auth.WalletAuth(), func(context *gin.Context) {
		bodyData := helpers.DecryptData(context, true)

		var data SetBeneficiaryData
		err := json.Unmarshal([]byte(bodyData), &data)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusBadRequest, err)
			return
		}

		err = config.SetBeneficiaryAddr(data.BeneficiaryAddr, config.Parameters.AllowEmptyBeneficiaryAddress)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusBadRequest, err)
			return
		}

		respData := helpers.EncryptData(context, true, gin.H{
			"beneficiaryAddr":        data.BeneficiaryAddr,
			"currentBeneficiaryAddr": config.Parameters.BeneficiaryAddr,
		})

		context.JSON(http.StatusOK, gin.H{
			"data": respData,
		})
		return

	})

}
