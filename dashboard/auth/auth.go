package auth

import (
	"bytes"
	"errors"
	"github.com/gin-gonic/gin"
	. "github.com/nknorg/nkn/common"
	serviceConfig "github.com/nknorg/nkn/dashboard/config"
	"github.com/nknorg/nkn/dashboard/helpers"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
	"net/http"
	"strconv"
	"time"
)

func verifyPasswordKey(passwordKey []byte, passwordHash []byte) bool {
	password := BytesToHexString(passwordHash)
	tick := time.Now().Unix()
	padding := int64(serviceConfig.UnixRange / 2)
	for i := tick - padding; i < tick+padding; i++ {
		if bytes.Equal(helpers.HmacSha256(password, strconv.FormatInt(i, 10)), passwordKey) {
			return true
		}
	}
	return false
}

// request filter: header["Authorization"] = (passwordhash)
func WalletAuth() gin.HandlerFunc {
	return func(context *gin.Context) {
		auth := context.GetHeader("Authorization")

		if auth == "" {
			context.AbortWithError(http.StatusUnauthorized, errors.New("401 Unauthorized"))
			return
		}
		wallet, exists := context.Get("wallet")
		if !exists {
			context.AbortWithError(http.StatusInternalServerError, errors.New("wallet is not init"))
			return
		}

		passwordKey, err := HexStringToBytes(auth)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusForbidden, err)
			return
		}
		passwordKeyHash, err := HexStringToBytes(wallet.(*vault.WalletImpl).Data.PasswordHash)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		if ok := verifyPasswordKey(passwordKey, passwordKeyHash[:]); !ok {
			context.AbortWithError(http.StatusForbidden, errors.New("403 Forbidden"))
			return
		}

		context.Next()
	}
}
