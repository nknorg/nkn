package cli

import (
	"math/rand"
	"time"

	"nkn/common/config"
	"nkn/common/log"
	"nkn/crypto"
)

func init() {
	log.Init()
	crypto.SetAlg(config.Parameters.EncryptAlg)
	//seed transaction nonce
	rand.Seed(time.Now().UnixNano())
}
