package auth

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"github.com/gin-gonic/gin"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
	"net/http"
)

func verifyPasswordKey(passwordKey []byte, passwordHash []byte) bool {
	keyHash := sha256.Sum256(passwordKey)
	if !bytes.Equal(passwordHash, keyHash[:]) {
		return false
	}

	return true
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
