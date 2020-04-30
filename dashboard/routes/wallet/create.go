package wallet

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	serviceConfig "github.com/nknorg/nkn/dashboard/config"
	"github.com/nknorg/nkn/dashboard/helpers"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/password"
	"github.com/nknorg/nkn/vault"
)

type CreateWalletData struct {
	Password        string `form:"password" binding:"required"`
	BeneficiaryAddr string `form:"beneficiaryAddr"`
}

func WalletCreateRouter(router *gin.RouterGroup) {
	router.POST("/wallet/create", func(context *gin.Context) {
		bodyData := helpers.DecryptData(context, false)

		var data CreateWalletData
		err := json.Unmarshal([]byte(bodyData), &data)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusBadRequest, err)
			return
		}

		_, exists := context.Get("wallet")
		if exists {
			log.WebLog.Error("Wallet file exists.")
			context.AbortWithError(http.StatusInternalServerError, errors.New("wallet file exists"))
			return
		}

		if config.Parameters.WebGuiCreateWallet {
			err = config.SetBeneficiaryAddr(data.BeneficiaryAddr, config.Parameters.AllowEmptyBeneficiaryAddress)
			if err != nil {
				log.WebLog.Error(err)
				context.AbortWithError(http.StatusBadRequest, err)
				return
			}
		}

		_, err = vault.NewWallet(config.Parameters.WalletFile, []byte(data.Password))
		if err != nil {
			log.WebLog.Error("Create wallet error: ", err)
			context.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		password.Passwd = data.Password
		err = password.SavePassword(password.Passwd)
		if err != nil {
			log.WebLog.Error("Save wallet error: ", err)
		}

		serviceConfig.Status = serviceConfig.Status | serviceConfig.SERVICE_STATUS_RUNNING

		context.JSON(http.StatusOK, "")
		return
	})

}
