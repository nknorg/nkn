package wallet

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/dashboard/helpers"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/password"
	"github.com/nknorg/nkn/vault"
)

type OpenWalletData struct {
	Password string `form:"password" binding:"required"`
}

func WalletOpenRouter(router *gin.RouterGroup) {
	router.POST("/wallet/open", func(context *gin.Context) {
		bodyData := helpers.DecryptData(context, false)
		var data OpenWalletData
		err := json.Unmarshal([]byte(bodyData), &data)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusBadRequest, err)
			return
		}

		wallet, exists := context.Get("wallet")
		if exists {
			err = wallet.(*vault.Wallet).VerifyPassword([]byte(data.Password))
			if err != nil {
				log.WebLog.Error("open wallet error: ", err)
				context.AbortWithError(http.StatusForbidden, err)
				return
			}
		} else {
			password.Passwd = data.Password
			_, err := vault.GetWallet()
			if err != nil {
				log.WebLog.Error("open wallet error: ", err)
				context.AbortWithError(http.StatusForbidden, err)
				return
			}

			err = password.SavePassword(password.Passwd)
			if err != nil {
				log.WebLog.Error("save wallet error: ", err)
			}
		}
		context.JSON(http.StatusOK, "")
		return
	})

}
