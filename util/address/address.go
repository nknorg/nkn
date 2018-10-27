package address

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"

	"github.com/nknorg/nkn/util/log"
)

// ParseClientAddress parse an address string to Chord address and public key
func ParseClientAddress(addrStr string) ([]byte, []byte, error) {
	clientID := sha256.Sum256([]byte(addrStr))

	substrings := strings.Split(addrStr, ".")
	pubKeyStr := substrings[len(substrings)-1]
	pubKey, err := hex.DecodeString(pubKeyStr)
	if err != nil {
		log.Error("Invalid public key string converting to hex")
		return nil, nil, err
	}

	return clientID[:], pubKey, nil
}

// ShouldRejectAddr returns if remoteAddr should be rejected by localAddr
func ShouldRejectAddr(localAddr, remoteAddr string) bool {
	localAddress, err := url.Parse(localAddr)
	if err != nil {
		return false
	}

	remoteAddress, err := url.Parse(remoteAddr)
	if err != nil {
		return false
	}

	if localAddress.Hostname() != remoteAddress.Hostname() && localAddress.Port() != remoteAddress.Port() {
		return true
	}

	return false
}
