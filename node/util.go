package node

import (
	"bytes"
	"encoding/hex"
	"net"
	"strings"
	"text/template"
)

type TlsDomain struct {
	DashedIP string
}

func (w *TlsDomain) IpToDomain(domainTemplate string) (string, error) {
	t := template.New("Wss Domain Template")
	t, err := t.Parse(domainTemplate)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	err = t.Execute(buf, w)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func GetDefaultDomainFromIP(hostname string, domainTemplate string) (string, error) {
	addr := net.ParseIP(hostname)
	if addr == nil {
		return hostname, nil
	}
	tlsDomain := &TlsDomain{DotToDash(hostname)}

	domainName, err := tlsDomain.IpToDomain(domainTemplate)
	if err != nil {
		return "", err
	}
	return domainName, nil
}

func chordIDToNodeID(chordID []byte) string {
	return hex.EncodeToString(chordID)
}

func DotToDash(ip string) string {
	return strings.ReplaceAll(ip, ".", "-")
}
