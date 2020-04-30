package wallet

import (
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/dashboard/auth"
	"github.com/nknorg/nkn/dashboard/helpers"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
)

func StatusRouter(router *gin.RouterGroup) {
	router.GET("/current-wallet/status", func(context *gin.Context) {
		wallet, exists := context.Get("wallet")

		if exists {
			account, err := wallet.(*vault.Wallet).GetDefaultAccount()
			if err != nil {
				log.WebLog.Error("Get wallet account error: ", err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			address, err := account.ProgramHash.ToAddress()
			if err != nil {
				log.WebLog.Error("Get wallet address error: ", err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			defaultLedger, err := chain.GetDefaultLedger()
			if err != nil {
				log.WebLog.Error("Database has not been initialized.")
				context.AbortWithError(http.StatusInternalServerError, errors.New("database has not been initialized"))
				return
			}

			balance := defaultLedger.Store.GetBalance(account.ProgramHash)

			data := helpers.EncryptData(context, true, gin.H{
				"balance":   balance.String(),
				"address":   address,
				"publicKey": hex.EncodeToString(account.PublicKey),
			})

			context.JSON(http.StatusOK, gin.H{
				"data": data,
			})
			return
		} else {
			log.WebLog.Error("Wallet has not been initialized.")
			context.AbortWithError(http.StatusInternalServerError, errors.New("wallet has not been initialized"))
			return
		}

	})

	router.GET("/current-wallet/details", auth.WalletAuth(), func(context *gin.Context) {
		wallet, exists := context.Get("wallet")
		if exists {
			account, err := wallet.(*vault.Wallet).GetDefaultAccount()
			if err != nil {
				log.WebLog.Error("get wallet account error: ", err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			data := helpers.EncryptData(context, true, gin.H{
				"secretSeed": hex.EncodeToString(crypto.GetSeedFromPrivateKey(account.PrivateKey)),
			})
			context.JSON(http.StatusOK, gin.H{
				"data": data,
			})
			return
		} else {
			log.WebLog.Error("wallet has not been initialized.")
			context.AbortWithError(http.StatusInternalServerError, errors.New("wallet has not been initialized."))
			return
		}

	})

}
