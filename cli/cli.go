package cli

import (
	"math/rand"
	"time"

	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/util/config"
)

func init() {
	crypto.SetAlg(config.EncryptAlg) // select signature algorithm
	rand.Seed(time.Now().UnixNano()) // seed transaction nonce
}
