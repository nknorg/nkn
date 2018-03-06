package cli

import (
	"math/rand"
	"time"

	"nkn-core/common/config"
	"nkn-core/common/log"
	"nkn-core/crypto"
)

func init() {
	log.Init()
	crypto.SetAlg(config.Parameters.EncryptAlg)
	//seed transaction nonce
	rand.Seed(time.Now().UnixNano())
}
