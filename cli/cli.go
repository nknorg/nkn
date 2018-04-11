package cli

import (
	"math/rand"
	"time"

	"github.com/nknorg/nkn/common/config"
	"github.com/nknorg/nkn/common/log"
	"github.com/nknorg/nkn/crypto"
)

func init() {
	log.Init()
	crypto.SetAlg(config.Parameters.EncryptAlg)
	//seed transaction nonce
	rand.Seed(time.Now().UnixNano())
}
