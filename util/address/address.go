package address

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/url"
	"strings"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/log"
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

// GenChordID generates an ID for the node
func GenChordID(host string) []byte {
	hash := sha256.New()
	hash.Write([]byte(host))
	return hash.Sum(nil)
}

func MakeAddressString(pubKey []byte, identifier string) string {
	var result strings.Builder
	if identifier != "" {
		result.WriteString(identifier)
		result.WriteString(".")
	}
	result.WriteString(common.BytesToHexString(pubKey))

	return result.String()
}

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
