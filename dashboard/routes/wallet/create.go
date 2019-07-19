package wallet

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	. "github.com/nknorg/nkn/common"
	serviceConfig "github.com/nknorg/nkn/dashboard/config"
	"github.com/nknorg/nkn/dashboard/helpers"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/password"
	"github.com/nknorg/nkn/vault"
	"net/http"
)

type CreateWalletData struct {
	Password        string `form:"password" binding:"required"`
	BeneficiaryAddr string `form:"beneficiaryAddr"`
}

func WalletCreateRouter(router *gin.RouterGroup) {
	router.POST("/wallet/create", func(context *gin.Context) {
		bodyData := helpers.DecryptData(context)

		var data CreateWalletData
		err := json.Unmarshal([]byte(bodyData), &data)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusBadRequest, err)
			return
		}

		_, exists := context.Get("wallet")
		if exists {
			log.WebLog.Error("wallet file exists.")
			context.AbortWithError(http.StatusInternalServerError, errors.New("wallet file exists."))
			return
		}

		if config.Parameters.WebGuiCreateWallet && !config.Parameters.AllowEmptyBeneficiaryAddress {
			if data.BeneficiaryAddr == "" {
				log.WebLog.Error("beneficiary address is empty.")
				context.AbortWithError(http.StatusBadRequest, errors.New("beneficiary address is empty."))
				return
			}

			_, err = ToScriptHash(data.BeneficiaryAddr)
			if err != nil {
				log.WebLog.Errorf("parse BeneficiaryAddr error: %v", err)
				context.AbortWithError(http.StatusBadRequest, err)
				return
			}

			file, err := config.OpenConfigFile()
			if err != nil {
				log.WebLog.Error("Config file not exists.")
				context.AbortWithError(http.StatusInternalServerError, errors.New("Config file not exists."))
				return
			}
			var configuration map[string]interface{}
			err = json.Unmarshal(file, &configuration)
			if err != nil {
				log.WebLog.Error(err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			// set beneficiary address
			configuration["BeneficiaryAddr"] = data.BeneficiaryAddr

			bytes, err := json.MarshalIndent(&configuration, "", "    ")
			if err != nil {
				log.WebLog.Error(err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			err = config.WriteConfigFile(bytes)
			if err != nil {
				log.WebLog.Error(err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			config.Parameters.BeneficiaryAddr = data.BeneficiaryAddr
		}

		_, err = vault.NewWallet(config.Parameters.WalletFile, []byte(data.Password), true)
		if err != nil {
			log.WebLog.Error("create wallet error: ", err)
			context.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		password.Passwd = data.Password
		err = password.SavePassword()
		if err != nil {
			log.WebLog.Error("save wallet password error: ", err)
			context.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		serviceConfig.Status = serviceConfig.SERVICE_STATUS_RUNNING

		context.JSON(http.StatusOK, "")
		return

	})

}
