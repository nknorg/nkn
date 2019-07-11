package node

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/dashboard/auth"
	"github.com/nknorg/nkn/dashboard/helpers"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"io/ioutil"
	"net/http"
	"os"
)

type SetBeneficiaryData struct {
	BeneficiaryAddr string `form:"beneficiaryAddr" binding:"required"`
}

func BeneficiaryRouter(router *gin.RouterGroup) {
	router.PUT("/node/beneficiary", auth.WalletAuth(), func(context *gin.Context) {
		bodyData := helpers.DecryptData(context)

		var data SetBeneficiaryData
		err := json.Unmarshal([]byte(bodyData), &data)
		if err != nil {
			log.WebLog.Error(err)
			context.AbortWithError(http.StatusBadRequest, err)
			return
		}

		if data.BeneficiaryAddr != "" {
			_, err = ToScriptHash(data.BeneficiaryAddr)
			if err != nil {
				log.WebLog.Errorf("parse BeneficiaryAddr error: %v", err)
				context.AbortWithError(http.StatusBadRequest, err)
				return
			}
		}

		configFile := config.ConfigFile
		if configFile == "" {
			configFile = config.DefaultConfigFile
		}
		if _, err := os.Stat(configFile); err == nil {
			file, err := ioutil.ReadFile(configFile)
			if err != nil {
				log.WebLog.Error("Read config file error:", err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			// Remove the UTF-8 Byte Order Mark
			file = bytes.TrimPrefix(file, []byte("\xef\xbb\xbf"))

			var configuration map[string]interface{}
			err = json.Unmarshal(file, &configuration)
			if err != nil {
				log.WebLog.Error(err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			// set beneficiary address
			configuration["BeneficiaryAddr"] = data.BeneficiaryAddr

			bytes, err := json.MarshalIndent(&configuration, "", "    ")
			if err != nil {
				log.WebLog.Error(err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			err = ioutil.WriteFile(configFile, bytes, 0666)
			if err != nil {
				log.WebLog.Error(err)
				context.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			data := helpers.EncryptData(context, gin.H{
				"beneficiaryAddr":        configuration["BeneficiaryAddr"],
				"currentBeneficiaryAddr": config.Parameters.BeneficiaryAddr,
			})

			context.JSON(http.StatusOK, gin.H{
				"data": data,
			})
			return
		} else {
			log.WebLog.Error("Config file not exists.")
			context.AbortWithError(http.StatusInternalServerError, errors.New("Config file not exists."))
			return
		}

	})

}
