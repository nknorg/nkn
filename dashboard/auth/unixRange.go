package auth

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	serviceConfig "github.com/nknorg/nkn/dashboard/config"
	"github.com/nknorg/nkn/util/log"
	"net/http"
	"strconv"
)

func verifyUnix(unix int64) bool {
	padding := int64(serviceConfig.UnixRange / 2)
	min := unix - padding
	max := unix + padding
	if min < unix && unix < max {
		return true
	}
	return false
}

func UnixRangeAuth() gin.HandlerFunc {
	return func(context *gin.Context) {
		unix := context.GetHeader("Unix")
		tick, err := strconv.ParseInt(unix, 10, 64)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusInternalServerError, err)
		}
		valid := verifyUnix(tick)
		fmt.Println(valid)
		if unix == "" {
			context.AbortWithError(http.StatusBadRequest, errors.New("400 Bad request"))
			return
		}

		context.Next()
	}
}
