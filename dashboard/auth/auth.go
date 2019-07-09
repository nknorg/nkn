package auth

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"github.com/gin-contrib/sessions"
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

func verifyPasswordKey(passwordKey []byte, passwordHash []byte, token string, seed string) bool {
	tick := time.Now().Unix()
	padding := int64(serviceConfig.UnixRange)
	hash := sha256.Sum224([]byte(BytesToHexString(passwordHash)))
	for i := tick - padding; i < tick+padding; i++ {
		hex, err := helpers.AesEncrypt(BytesToHexString(hash[:]), seed+token+strconv.FormatInt(i, 10))
		if err != nil {
			log.WebLog.Error(err)
			return false
		}
		hexByte, err := HexStringToBytes(hex)
		if err != nil {
			log.WebLog.Error(err)
			return false
		}
		if bytes.Equal(hexByte, passwordKey) {
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

		//seed, exists := context.Get("seed")
		//if !exists {
		//	context.AbortWithError(http.StatusInternalServerError, errors.New("wallet is not init"))
		//	return
		//}

		session := sessions.Default(context)
		token := session.Get("token")

		if token == nil {
			context.AbortWithError(http.StatusForbidden, errors.New("403 Forbidden"))
			return
		}
		if ok := verifyPasswordKey(passwordKey, passwordKeyHash, token.(string), ""); !ok {
			context.AbortWithError(http.StatusForbidden, errors.New("403 Forbidden"))
			return
		}

		context.Next()
	}
}
