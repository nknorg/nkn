package wallet

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/dashboard/auth"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
)

type DownloadWalletData struct {
	Password        string `form:"password" binding:"required"`
	BeneficiaryAddr string `form:"beneficiaryAddr"`
}

func WalletDownloadRouter(router *gin.RouterGroup) {
	router.GET("/wallet/download", auth.WalletAuth(), func(context *gin.Context) {

		wallet, exists := context.Get("wallet")
		if !exists {
			log.WebLog.Error("Wallet file not exists.")
			context.AbortWithError(http.StatusInternalServerError, errors.New("wallet file not exists"))
			return
		}

		bytes, err := json.MarshalIndent(&wallet.(*vault.Wallet).WalletData, "", "    ")
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		context.Writer.WriteHeader(http.StatusOK)
		context.Header("Content-Disposition", "attachment; filename=wallet.json")
		context.Header("Content-Type", "application/json")
		context.Header("Accept-Length", fmt.Sprintf("%d", len(bytes)))
		context.Writer.Write(bytes)

		return

	})

}
