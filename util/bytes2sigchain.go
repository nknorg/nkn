package main

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/nknorg/nkn/pb"
)

// Base64ToHex convert base64 string input to hex string output
func Base64ToHex(in []byte) (out []byte, err error) { // input format should be ':"balabala..."'
	const prefix, suffix = ":\"", "\""
	start, end := len(prefix), len(in)-len(suffix) // b64Str len not contain :" prefix and " suffix

	// Decode Base64 string to bin
	bin := make([]byte, base64.StdEncoding.DecodedLen(end-start))
	n, err := base64.StdEncoding.Decode(bin, in[start:end])
	if err != nil {
		fmt.Fprintf(os.Stderr, "base64 decode %s met erro %v\n", in[start:end], err)
		return []byte{}, err
	}

	// Encode bin to hex string
	hexStr := make([]byte, 2*len(bin[:n]))
	hex.Encode(hexStr, bin[:n])

	// return strcat
	out = append(out, prefix...)       // append :" prefix
	out = append(out, hexStr...)       // append hexStr
	return append(out, suffix...), err // append " suffix
}

// Base64Handler will find all matching with pattern in input, and replace them with hex string
func Base64Handler(pattern *regexp.Regexp, out io.Writer, in []byte) {
	seek := 0                                        // init cursor
	for _, s := range pattern.FindAllIndex(in, -1) { // foreach matching
		out.Write(in[seek:s[0]]) // copy string between two matching

		// append hex string if convert successful or append orig in failed
		if str, err := Base64ToHex(in[s[0]:s[1]]); err == nil {
			out.Write(str)
		} else {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			out.Write(in[s[0]:s[1]])
		}
		seek = s[1] // update cursor
	}

	out.Write(in[seek:]) // copy remaining string
}

func main() {
	// regexp pattern `:"RFC4648"`
	re := regexp.MustCompile(":\"(?:[a-zA-Z0-9+\\/]{4})*(?:|(?:[a-zA-Z0-9+\\/]{3}=)|(?:[a-zA-Z0-9+\\/]{2}==)|(?:[a-zA-Z0-9+\\/]{1}===))\"")
	for {
		l, err := bufio.NewReader(os.Stdin).ReadString('\n')
		switch err {
		case nil: // readLine successful from stdin
		case io.EOF:
			return
		default:
			fmt.Fprintf(os.Stderr, "%v\n", err)
			continue // read current line error, skip & next
		}

		var buf, jStr []byte
		if buf, err = hex.DecodeString(strings.TrimRight(l, "\n")); err == nil { // hexStr to bin
			commitTxn := &pb.Commit{}
			if err = commitTxn.Unmarshal(buf); err == nil { // bin to pb struct of commitType txn
				// submitter, _ := new(common.Uint160).SetBytes(commitTxn.Submitter).ToAddress()
				sc := &pb.SigChain{}
				if err = sc.Unmarshal(commitTxn.SigChain); err == nil { // bin to pb struct of sigChain
					js := map[string]interface{}{"SigChain": sc, "Submitter": commitTxn.Submitter}
					if jStr, err = json.Marshal(js); err == nil { // to json string
						Base64Handler(re, os.Stdout, jStr) // replace all in jStr to stdout
						os.Stdout.WriteString("\n")
						continue
					}
				}
			}
		}
		fmt.Fprintf(os.Stderr, "%v\n", err) // any error from above
	}
}
