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

func verifyPasswordKey(passwordKey []byte, passwordHash []byte, token string) bool {
	tick := time.Now().Unix()
	padding := int64(serviceConfig.UnixRange)
	pwdData := []byte(BytesToHexString(passwordHash))
	hash := sha256.Sum224(pwdData)
	for i := tick - padding; i < tick+padding; i++ {
		seedHash := BytesToHexString(helpers.HmacSha256([]byte(BytesToHexString(hash[:])), []byte(token+strconv.FormatInt(i, 10))))
		hexHash, err := helpers.AesEncrypt(BytesToHexString(hash[:]), seedHash)
		if err != nil {
			log.WebLog.Error(err)
			return false
		}
		hexByte, err := HexStringToBytes(hexHash)
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
			context.AbortWithError(http.StatusInternalServerError, errors.New("wallet has not been initialized"))
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

		session := sessions.Default(context)
		token := session.Get("token")

		if token == nil {
			context.AbortWithError(http.StatusForbidden, errors.New("403 Forbidden"))
			return
		}
		if ok := verifyPasswordKey(passwordKey, passwordKeyHash, token.(string)); !ok {
			context.AbortWithError(http.StatusForbidden, errors.New("403 Forbidden"))
			return
		}

		context.Next()
	}
}
