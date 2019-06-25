package wallet

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type WalletRouter struct {
}

func (walletRouter *WalletRouter) Router(router *gin.RouterGroup) {
	router.GET("/wallet/status", func(context *gin.Context) {
		wallet, exists := context.Get("wallet")
		if exists {
			context.JSON(http.StatusOK, wallet)
			return
		}

		context.JSON(http.StatusOK, gin.H{
			"status": "...",
		})
	})
}
