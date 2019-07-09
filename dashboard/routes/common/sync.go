package common

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
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
}
