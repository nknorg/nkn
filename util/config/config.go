package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
)

const (
	MINGENBLOCKTIME        = 2
	DEFAULTGENBLOCKTIME    = 6
	DefaultConfigFilename  = "./config.json"
	DefaultBookKeeperCount = 4
	DefaultMultiCoreNum    = 4
)

var Version string

type Configuration struct {
	Magic          int64    `json:"Magic"`
	Version        int      `json:"Version"`
	SeedList       []string `json:"SeedList"`
	BookKeepers    []string `json:"BookKeepers"`
	HttpRestPort   int      `json:"HttpRestPort"`
	RestCertPath   string   `json:"RestCertPath"`
	RestKeyPath    string   `json:"RestKeyPath"`
	HttpInfoPort   uint16   `json:"HttpInfoPort"`
	HttpInfoStart  bool     `json:"HttpInfoStart"`
	HttpWsPort     int      `json:"HttpWsPort"`
	HttpJsonPort   int      `json:"HttpJsonPort"`
	NodePort       int      `json:"NodePort"`
	NodeType       string   `json:"NodeType"`
	WebSocketPort  int      `json:"WebSocketPort"`
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
	ChordPort      int      `json:"ChordPort"`
}

var Parameters *Configuration

func init() {
	file, err := ioutil.ReadFile(DefaultConfigFilename)
	if err != nil {
		log.Fatalln("Config file error: ", err)
	}
	// Remove the UTF-8 Byte Order Mark
	file = bytes.TrimPrefix(file, []byte("\xef\xbb\xbf"))

	config := Configuration{}
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Fatalln("Unmarshal config file error: ", err)
	}
	err = check(&config)
	if err != nil {
		log.Fatalln("invalid config file: ", err)
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
