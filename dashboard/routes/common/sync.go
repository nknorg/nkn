package common

import (
	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/v2/config"
	serviceConfig "github.com/nknorg/nkn/v2/dashboard/config"
	"net/http"
	"time"
)

func SyncRouter(router *gin.RouterGroup) {
	router.GET("/sync/unix", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{
			"unix": time.Now().Unix(),
		})
		return
	})

	router.GET("/sync/token", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{
			"token": serviceConfig.Token,
			"unix":  time.Now().Unix(),
		})
		return
	})

	router.GET("/sync/status", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{
			"isNodeInit":                   serviceConfig.IsNodeInit,
			"isWalletInit":                 serviceConfig.IsWalletInit,
			"status":                       serviceConfig.Status,
			"beneficiaryAddr":              config.Parameters.BeneficiaryAddr,
			"webGuiCreateWallet":           config.Parameters.WebGuiCreateWallet,
			"allowEmptyBeneficiaryAddress": config.Parameters.AllowEmptyBeneficiaryAddress,
		})
		return
	})
}
