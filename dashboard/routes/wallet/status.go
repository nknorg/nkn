package wallet

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/chain"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/dashboard/auth"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
	"net/http"
)

type WalletRouter struct {
}

func (walletRouter *WalletRouter) Router(router *gin.RouterGroup) {
	router.GET("/current-wallet/status", func(context *gin.Context) {
		wallet, exists := context.Get("wallet")

		if exists {
			account, err := wallet.(vault.Wallet).GetDefaultAccount()
			if err != nil {
				log.WebLog.Error("get wallet account error: ", err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			address, err := account.ProgramHash.ToAddress()
			if err != nil {
				log.WebLog.Error("get wallet address error: ", err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			pg, err := ToScriptHash(address)
			if err != nil {
				log.WebLog.Error("get wallet address error: ", err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			balance := chain.DefaultLedger.Store.GetBalance(pg)
			context.JSON(http.StatusOK, gin.H{
				"balance":   balance.String(),
				"address":   address,
				"publicKey": BytesToHexString(account.PublicKey.EncodePoint()),
			})
			return
		} else {
			log.WebLog.Error("wallet has not init.")
			context.AbortWithError(http.StatusInternalServerError, errors.New("wallet has not init."))
			return
		}

	})

	router.GET("/current-wallet/details", auth.WalletAuth(), func(context *gin.Context) {
		wallet, exists := context.Get("wallet")
		if exists {
			account, err := wallet.(vault.Wallet).GetDefaultAccount()
			if err != nil {
				log.WebLog.Error("get wallet account error: ", err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			context.JSON(http.StatusOK, gin.H{
				"privateKey": BytesToHexString(account.PrivateKey),
			})
			return
		} else {
			log.WebLog.Error("wallet has not init.")
			context.AbortWithError(http.StatusInternalServerError, errors.New("wallet has not init."))
			return
		}

	})

}
