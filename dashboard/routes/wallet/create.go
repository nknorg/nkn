package wallet

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/dashboard/helpers"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/password"
	"github.com/nknorg/nkn/vault"
	"net/http"
)

type CreateWalletData struct {
	Password string `form:"password" binding:"required"`
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
		} else {
			_, err := vault.NewWallet(config.Parameters.WalletFile, []byte(data.Password), true)
			if err != nil {
				log.WebLog.Error("create wallet error: ", err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			password.Passwd = data.Password
			context.JSON(http.StatusOK, "")
			return
		}

	})

}
