package node

import (
	"bytes"
	"encoding/hex"
	"strings"
	"text/template"

	"github.com/nknorg/nkn/util/config"
)

type WssDomain struct {
	DashedIP string
}

func chordIDToNodeID(chordID []byte) string {
	return hex.EncodeToString(chordID)
}

func DotToDash(ip string) string {
	return strings.ReplaceAll(ip, ".", "-")
}

func (w *WssDomain) IpToDomain() (string, error) {
	t := template.New("Wss Domain Template")
	t, err := t.Parse(config.Parameters.TlsDomain)
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
