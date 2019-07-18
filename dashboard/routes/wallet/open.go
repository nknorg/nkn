package wallet

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/dashboard/helpers"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/password"
	"github.com/nknorg/nkn/vault"
	"net/http"
)

type OpenWalletData struct {
	Password string `form:"password" binding:"required"`
}

func WalletOpenRouter(router *gin.RouterGroup) {
	router.POST("/wallet/open", func(context *gin.Context) {
		bodyData := helpers.DecryptData(context)

		var data OpenWalletData
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
			password.Passwd = data.Password
			_, err := vault.GetWallet()
			if err != nil {
				log.WebLog.Error("open wallet error: ", err)
				context.AbortWithError(http.StatusForbidden, err)
				return
			}
			context.JSON(http.StatusOK, "")
			return
		}

	})

}
