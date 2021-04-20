package address

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"

	"github.com/nknorg/nkn/v2/util/log"
)

var privateIPBlocks []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local address
	} {
		_, block, _ := net.ParseCIDR(cidr)
		privateIPBlocks = append(privateIPBlocks, block)
	}
}

// IsPrivateIP returns if an IP address is a private IP address
func IsPrivateIP(ip net.IP) bool {
	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

func MakeAddressString(pubKey []byte, identifier string) string {
	var result strings.Builder
	if identifier != "" {
		result.WriteString(identifier)
		result.WriteString(".")
	}
	result.WriteString(hex.EncodeToString(pubKey))

	return result.String()
}

// ParseClientAddress parse an address string to Chord address, public key, and
// identifier
func ParseClientAddress(addrStr string) ([]byte, []byte, string, error) {
	clientID := sha256.Sum256([]byte(addrStr))

	substrings := strings.Split(addrStr, ".")
	pubKeyStr := substrings[len(substrings)-1]
	pubKey, err := hex.DecodeString(pubKeyStr)
	if err != nil {
		log.Error("Invalid public key string converting to hex")
		return nil, nil, "", err
	}

	identifier := strings.Join(substrings[:len(substrings)-1], ".")

	return clientID[:], pubKey, identifier, nil
}

// AssembleClientAddress returns the client address string from identifier and
// pubkey
func AssembleClientAddress(identifier string, pubkey []byte) string {
	addr := hex.EncodeToString(pubkey)
	if len(identifier) > 0 {
		addr = identifier + "." + addr
	}
	return addr
}
