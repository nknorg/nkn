package wallet

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/dashboard/helpers"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/password"
	"github.com/nknorg/nkn/vault"
	"net/http"
)

func verifyPasswordKey(passwordKey []byte, passwordHash []byte) bool {
	keyHash := sha256.Sum256(passwordKey)
	if !bytes.Equal(passwordHash, keyHash[:]) {
		fmt.Println("error: password wrong")
		return false
	}

	return true
}

type OpenWalletData struct {
	Password string `form:"password" binding:"required"`
}

func WalletOpenRouter(router *gin.RouterGroup) {
	router.POST("/wallet/open", func(context *gin.Context) {
		bodyData := helpers.DecryptData(context, false)
		var data OpenWalletData
		err := json.Unmarshal([]byte(bodyData), &data)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusBadRequest, err)
			return
		}

		wallet, exists := context.Get("wallet")
		if exists {
			passwordKey := crypto.ToAesKey([]byte(data.Password))
			passwordKeyHash, err := HexStringToBytes(wallet.(*vault.WalletImpl).Data.PasswordHash)
			if err != nil {
				log.WebLog.Error("open wallet error: ", err)
				context.AbortWithError(http.StatusForbidden, err)
				return
			}
			if ok := verifyPasswordKey(passwordKey, passwordKeyHash); !ok {
				log.WebLog.Error("open wallet error: ", errors.New("password wrong"))
				context.AbortWithError(http.StatusForbidden, errors.New("password wrong"))
				return
			}

		} else {
			password.Passwd = data.Password
			_, err := vault.GetWallet()
			if err != nil {
				log.WebLog.Error("open wallet error: ", err)
				context.AbortWithError(http.StatusForbidden, err)
				return
			}

			err = password.SavePassword(password.Passwd)
			if err != nil {
				log.WebLog.Error("save wallet error: ", err)
			}
		}
		context.JSON(http.StatusOK, "")
		return

	})

}
