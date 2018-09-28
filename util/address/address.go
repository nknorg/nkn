package address

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
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
	localHost, localPort, err := net.SplitHostPort(localAddr)
	if err != nil {
		return false
	}

	remoteHost, remoteport, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}

	if localHost != remoteHost && localPort != remoteport {
		return true
	}

	return false
}
