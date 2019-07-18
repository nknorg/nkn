package auth

import (
	"errors"
	"github.com/gin-gonic/gin"
	serviceConfig "github.com/nknorg/nkn/dashboard/config"
	"github.com/nknorg/nkn/util/log"
	"math"
	"net/http"
	"strconv"
	"time"
)

func verifyUnix(a int64, b int64) bool {
	return math.Abs(float64(a-b)) < serviceConfig.UnixRange
}

func UnixRangeAuth() gin.HandlerFunc {
	return func(context *gin.Context) {
		unix := context.GetHeader("Unix")

		if unix == "" {
			context.AbortWithError(http.StatusBadRequest, errors.New("400 Bad request"))
			return
		}

		tick, err := strconv.ParseInt(unix, 10, 64)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusInternalServerError, err)
		}
		now := time.Now().Unix()
		valid := verifyUnix(now, tick)

		if !valid {
			context.AbortWithError(http.StatusBadRequest, errors.New("400 Bad request"))
			return
		}

		context.Next()
	}
}
