package home

import "github.com/gin-gonic/gin"

type HomeRouter struct {
}

func (homeRouter *HomeRouter) Router(router *gin.Engine) {

	router.GET("/home", func(context *gin.Context) {
		context.JSON(200, gin.H{
			"message": "/home",
		})
	})
}

func (homeRouter *HomeRouter) RouterByGroup(router *gin.RouterGroup) {

	router.GET("/home", func(context *gin.Context) {
		context.JSON(200, gin.H{
			"message": "/home",
		})
	})
}
