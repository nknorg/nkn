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

	ipify "github.com/rdegges/go-ipify"
)

const (
	MINGENBLOCKTIME        = 2
	DEFAULTGENBLOCKTIME    = 6
	DefaultConfigFilename  = "./config.json"
	DefaultBookKeeperCount = 4
)

var (
	Version           string
	Parameters        *Configuration
	defaultParameters = &Configuration{
		Magic:         99281,
		Version:       1,
		ChordPort:     30000,
		NodePort:      30001,
		HttpWsPort:    30002,
		HttpJsonPort:  30003,
		LogLevel:      1,
		ConsensusType: "ising",
		SeedList: []string{
			"127.0.0.1:30000",
		},
	}
)

type Configuration struct {
	Magic                int64    `json:"Magic"`
	Version              int      `json:"Version"`
	SeedList             []string `json:"SeedList"`
	BookKeepers          []string `json:"BookKeepers"`
	RestCertPath         string   `json:"RestCertPath"`
	RestKeyPath          string   `json:"RestKeyPath"`
	RPCCert              string   `json:"RPCCert"`
	RPCKey               string   `json:"RPCKey"`
	HttpInfoPort         uint16   `json:"HttpInfoPort"`
	HttpInfoStart        bool     `json:"HttpInfoStart"`
	HttpWsPort           uint16   `json:"HttpWsPort"`
	HttpJsonPort         uint16   `json:"HttpJsonPort"`
	NodePort             uint16   `json:"NodePort"`
	LogLevel             int      `json:"LogLevel"`
	IsTLS                bool     `json:"IsTLS"`
	CertPath             string   `json:"CertPath"`
	KeyPath              string   `json:"KeyPath"`
	CAPath               string   `json:"CAPath"`
	GenBlockTime         uint     `json:"GenBlockTime"`
	EncryptAlg           string   `json:"EncryptAlg"`
	MaxLogSize           int64    `json:"MaxLogSize"`
	MaxTxInBlock         int      `json:"MaxTransactionInBlock"`
	MaxHdrSyncReqs       int      `json:"MaxConcurrentSyncHeaderReqs"`
	ConsensusType        string   `json:"ConsensusType"`
	ChordPort            uint16   `json:"ChordPort"`
	GenesisBlockProposer []string `json:"GenesisBlockProposer"`
	Hostname             string   `json:"Hostname"`
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

	if config.Hostname == "" {
		ip, err := ipify.GetIp()
		if err != nil {
			log.Printf("Couldn't get my IP address: %v", err)
			ip = "127.0.0.1"
		}
		config.Hostname = ip
	}

	config.IncrementPort()

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
		return fmt.Errorf("invalid consensus type %s in config file\n", config.ConsensusType)
	}

	return nil
}

func findMinMaxPort(array []uint16) (uint16, uint16) {
	var max = array[0]
	var min = array[0]
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

func (config *Configuration) IncrementPort() {
	allPorts := []uint16{
		config.ChordPort,
		config.NodePort,
		config.HttpWsPort,
		config.HttpJsonPort,
	}
	minPort, maxPort := findMinMaxPort(allPorts)
	step := maxPort - minPort + 1
	var delta uint16
	for {
		conn, err := net.Listen("tcp", ":"+strconv.Itoa(int(config.ChordPort+delta)))
		if err == nil {
			conn.Close()
			break
		}
		delta += step
	}
	config.ChordPort += delta
	config.NodePort += delta
	config.HttpWsPort += delta
	config.HttpJsonPort += delta
}
