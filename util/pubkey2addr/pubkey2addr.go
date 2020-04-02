package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/program"
)

func Key2Address(key string) (addr string, err error) {
	if pk, err := hex.DecodeString(key); err == nil {
		if err = crypto.CheckPublicKey(pk); err == nil {
			if redeemHash, err := program.CreateProgramHash(pk); err == nil {
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
