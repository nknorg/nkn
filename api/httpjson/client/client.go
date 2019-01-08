package client

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

// Call sends RPC request to server
func Call(address string, method string, id interface{}, params map[string]interface{}) ([]byte, error) {
	data, err := json.Marshal(map[string]interface{}{
		"method": method,
		"id":     id,
		"params": params,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Marshal JSON request: %v\n", err)
		return nil, err
	}
	resp, err := http.Post(address, "application/json", strings.NewReader(string(data)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "POST request: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GET response: %v\n", err)
		return nil, err
	}

	return body, nil
}

func GetRemoteBlkHeight(remote string) (uint32, error) {
	resp, err := Call(remote, "getlatestblockheight", 0, map[string]interface{}{})
	if err != nil {
		return 0, err
	}

	var ret struct {
		Jsonrpc string `json:"jsonrpc"`
		Id      uint   `json:"id"`
		Result  uint32 `json:"result"`
	}
	if err := json.Unmarshal(resp, &ret); err != nil {
		log.Println(err)
		return 0, err
	}

	return ret.Result, nil
}

func FindSuccessorAddrs(remote string, key []byte) ([]string, error) {
	resp, err := Call(remote, "findsuccessoraddrs", 0, map[string]interface{}{
		"key": hex.EncodeToString(key),
	})
	if err != nil {
		return nil, err
	}
	log.Printf("FindSuccessorAddrs: %s\n", string(resp))

	var ret struct {
		Result []string `json:"result"`
	}
	if err := json.Unmarshal(resp, &ret); err != nil {
		log.Println(err)
		return nil, err
	}
	log.Printf("Successor Address: %v\n", ret)

	return ret.Result, nil
}
