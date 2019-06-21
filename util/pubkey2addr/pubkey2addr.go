package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/program"
)

func Key2Address(key string) (addr string, err error) {
	var k []byte
	var pk *crypto.PubKey
	var redeemHash common.Uint160

	if k, err = hex.DecodeString(key); err == nil {
		if pk, err = crypto.DecodePoint(k); err == nil {
			if redeemHash, err = program.CreateRedeemHash(pk); err == nil {
				return redeemHash.ToAddress()
			}
		}
	}
	return "", err
}

func main() {
	for _, key := range os.Args[1:] {
		addr, err := Key2Address(key)
		if err != nil {
			os.Stderr.WriteString(err.Error() + "\n")
			continue
		}
		fmt.Println(addr)
	}
}
