package auth

import (
	"errors"
	"github.com/gin-gonic/gin"
	"log"
)

func Auth() gin.HandlerFunc {
	return func(context *gin.Context) {
		log.Println("--------------------")
		auth := context.GetHeader("Authorization")
		log.Print(auth)
		if auth == "" {
			//context.JSON(403, gin.H{"message":"ooss"})
			context.AbortWithError(403, errors.New("auth error"))
			return
		}
		context.Next()
	}
}
