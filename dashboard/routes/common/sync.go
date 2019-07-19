package common

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	serviceConfig "github.com/nknorg/nkn/dashboard/config"
	"github.com/nknorg/nkn/util/config"
	"github.com/pborman/uuid"
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
		token := uuid.NewUUID().String()
		session := sessions.Default(context)
		session.Set("token", token)
		session.Save()

		context.JSON(http.StatusOK, gin.H{
			"token": token,
			"unix":  time.Now().Unix(),
		})
		return
	})

	router.GET("/sync/status", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{
			"isInit":                       serviceConfig.IsInit,
			"status":                       serviceConfig.Status,
			"beneficiaryAddr":              config.Parameters.BeneficiaryAddr,
			"webGuiCreateWallet":           config.Parameters.WebGuiCreateWallet,
			"allowEmptyBeneficiaryAddress": config.Parameters.AllowEmptyBeneficiaryAddress,
		})
		return
	})
}
