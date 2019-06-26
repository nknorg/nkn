package wallet

import (
	"encoding/hex"
	"github.com/gin-gonic/gin"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
	"net/http"
)

type WalletRouter struct {
}

func (walletRouter *WalletRouter) Router(router *gin.RouterGroup) {
	router.GET("/current-wallet/status", func(context *gin.Context) {
		var err error
		wallet, exists := context.Get("wallet")

		account, err := wallet.(vault.Wallet).GetDefaultAccount()
		if err != nil {
			log.WebLog.Error("get wallet account error: ", err)
			context.AbortWithError(http.StatusInternalServerError, err)
		}

		address, err := account.ProgramHash.ToAddress()
		if err != nil {
			log.WebLog.Error("get wallet address error: ", err)
			context.AbortWithError(http.StatusInternalServerError, err)
		}

		pg, err := common.ToScriptHash(address)
		if err != nil {
			log.WebLog.Error("get wallet address error: ", err)
			context.AbortWithError(http.StatusInternalServerError, err)
		}

		balance := chain.DefaultLedger.Store.GetBalance(pg)

		if exists {
			context.JSON(http.StatusOK, gin.H{
				"balance":   balance.String(),
				"address":   address,
				"publicKey": hex.EncodeToString(account.PublicKey.EncodePoint()),
			})
			return
		}

	})
}
