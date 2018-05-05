package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strconv"
)

const (
	MINGENBLOCKTIME        = 2
	DEFAULTGENBLOCKTIME    = 6
	DefaultConfigFilename  = "./config.json"
	DefaultBookKeeperCount = 4
	DefaultMultiCoreNum    = 4
)

var (
	Version           string
	Parameters        *Configuration
	defaultParameters = &Configuration{
		Magic:         99281,
		Version:       1,
		HttpRestPort:  10334,
		HttpWsPort:    10335,
		HttpJsonPort:  10336,
		NodePort:      10338,
		ChordPort:     10339,
		PrintLevel:    5,
		ConsensusType: "ising",
		SeedList: []string{
			"127.0.0.1:10339",
		},
	}
)

type Configuration struct {
	Magic          int64    `json:"Magic"`
	Version        int      `json:"Version"`
	SeedList       []string `json:"SeedList"`
	BookKeepers    []string `json:"BookKeepers"`
	HttpRestPort   uint16   `json:"HttpRestPort"`
	RestCertPath   string   `json:"RestCertPath"`
	RestKeyPath    string   `json:"RestKeyPath"`
	HttpInfoPort   uint16   `json:"HttpInfoPort"`
	HttpInfoStart  bool     `json:"HttpInfoStart"`
	HttpWsPort     uint16   `json:"HttpWsPort"`
	HttpJsonPort   uint16   `json:"HttpJsonPort"`
	NodePort       uint16   `json:"NodePort"`
	NodeType       string   `json:"NodeType"`
	PrintLevel     int      `json:"PrintLevel"`
	IsTLS          bool     `json:"IsTLS"`
	CertPath       string   `json:"CertPath"`
	KeyPath        string   `json:"KeyPath"`
	CAPath         string   `json:"CAPath"`
	GenBlockTime   uint     `json:"GenBlockTime"`
	EncryptAlg     string   `json:"EncryptAlg"`
	MaxLogSize     int64    `json:"MaxLogSize"`
	MaxTxInBlock   int      `json:"MaxTransactionInBlock"`
	MaxHdrSyncReqs int      `json:"MaxConcurrentSyncHeaderReqs"`
	ConsensusType  string   `json:"ConsensusType"`
	ChordPort      uint16   `json:"ChordPort"`
}

func init() {
	file, err := ioutil.ReadFile(DefaultConfigFilename)
	if err != nil {
		log.Printf("Config file error: %v, use default parameters.", err)
		Parameters = defaultParameters
		return
	}
	// Remove the UTF-8 Byte Order Mark
	file = bytes.TrimPrefix(file, []byte("\xef\xbb\xbf"))

	config := Configuration{}
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Printf("Unmarshal config file error: %v, use default parameters.", err)
		Parameters = defaultParameters
		return
	}
	err = check(&config)
	if err != nil {
		log.Printf("invalid config file: %v, use default parameters.", err)
		Parameters = defaultParameters
		return
	}

	Parameters = &config
}

func check(config *Configuration) error {
	switch config.ConsensusType {
	case "dbft":
		if len(config.BookKeepers) < DefaultBookKeeperCount {
			return errors.New("error config for dbft consensus, needs 4 BookKeepers at least")
		}
		fallthrough
	case "ising":
		if len(config.SeedList) == 0 {
			return errors.New("seed list in config file should not be blank")
		}
	default:
		fmt.Println(config.ConsensusType)
		return errors.New("consensus type in config file should not be blank")
	}

	return nil
}

func findMinMaxPort(array []uint16) (uint16, uint16) {
	var max uint16 = array[0]
	var min uint16 = array[0]
	for _, value := range array {
		if max < value {
			max = value
		}
		if min > value {
			min = value
		}
	}
	return min, max
}

func IncrementPort() {
	allPorts := []uint16{
		Parameters.HttpInfoPort,
		Parameters.HttpRestPort,
		Parameters.HttpWsPort,
		Parameters.HttpJsonPort,
		Parameters.NodePort,
		Parameters.ChordPort,
	}
	minPort, maxPort := findMinMaxPort(allPorts)
	step := maxPort - minPort + 1
	var delta uint16
	for {
		conn, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(int(Parameters.ChordPort+delta)))
		if err == nil {
			conn.Close()
			break
		}
		delta += step
	}
	Parameters.HttpInfoPort += delta
	Parameters.HttpRestPort += delta
	Parameters.HttpWsPort += delta
	Parameters.HttpJsonPort += delta
	Parameters.NodePort += delta
	Parameters.ChordPort += delta
}
