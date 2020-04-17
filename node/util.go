package node

import (
	"bytes"
	"encoding/hex"
	"strings"
	"text/template"
)

type TlsDomain struct {
	DashedIP string
}

func (w *TlsDomain) IpToDomain(domain string) (string, error) {
	t := template.New("Wss Domain Template")
	t, err := t.Parse(domain)
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

func chordIDToNodeID(chordID []byte) string {
	return hex.EncodeToString(chordID)
}

func DotToDash(ip string) string {
	return strings.ReplaceAll(ip, ".", "-")
}
