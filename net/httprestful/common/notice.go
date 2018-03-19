package common

import (
	Err "nkn-core/net/httprestful/error"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

var pushBlockFlag bool = true

func CheckPushBlock() bool {
	return pushBlockFlag
}

func SetPushBlockFlag(cmd map[string]interface{}) map[string]interface{} {
	resp := ResponsePack(Err.SUCCESS)
	open, ok := cmd["Open"].(bool)
	if !ok {
		resp["Error"] = Err.INVALID_PARAMS
		return resp
	}
	pushBlockFlag = open
	resp["Result"] = pushBlockFlag
	return resp
}

func PostRequest(cmd map[string]interface{}, url string) (map[string]interface{}, error) {

	var repMsg = make(map[string]interface{})

	data, err := json.Marshal(cmd)
	if err != nil {
		return repMsg, err
	}
	reqData := bytes.NewBuffer(data)
	transport := http.Transport{
		Dial: func(netw, addr string) (net.Conn, error) {
			conn, err := net.DialTimeout(netw, addr, time.Second*10)
			if err != nil {
				return nil, err
			}
			conn.SetDeadline(time.Now().Add(time.Second * 10))
			return conn, nil
		},
		DisableKeepAlives: false,
	}
	client := &http.Client{Transport: &transport}
	request, err := http.NewRequest("POST", url, reqData)
	if err != nil {
		return repMsg, err
	}
	request.Header.Set("Content-type", "application/json")

	response, err := client.Do(request)
	if response != nil {
		defer response.Body.Close()
		if response.StatusCode == 200 {
			body, _ := ioutil.ReadAll(response.Body)
			if err := json.Unmarshal(body, &repMsg); err == nil {
				return repMsg, err
			}
		}
	}

	if err != nil {
		return repMsg, err
	}

	return repMsg, err
}
