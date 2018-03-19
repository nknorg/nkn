package config

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"math/big"
	"time"
)

const (
	DefaultConfigFilename = "./config.json"
	MINGENBLOCKTIME       = 2
	DEFAULTGENBLOCKTIME   = 6
)

var Version string

type PowConfiguration struct {
	Switch           string `json:"Switch"`
	PayToAddr        string `json:"PayToAddr"`
	MiningSelfPort   int    `josn:"MiningSelfPort"`
	ProtocolVersion  int    `json:"ProtocolVersion"`
	Proxy            string `json:"Proxy"`
	AutoMining       bool   `json:"AutoMining"`
	MinTxFee         int    `json:"MinTxFee"`
}

type Configuration struct {
	Magic               uint32           `json:"Magic"`
	Version             int              `json:"Version"`
	SeedList            []string         `json:"SeedList"`
	HttpRestPort        int              `json:"HttpRestPort"`
	RestCertPath        string           `json:"RestCertPath"`
	RestKeyPath         string           `json:"RestKeyPath"`
	HttpInfoPort        uint16           `json:"HttpInfoPort"`
	HttpInfoStart       bool             `json:"HttpInfoStart"`
	HttpWsPort          int              `json:"HttpWsPort"`
	WsHeartbeatInterval time.Duration    `json:"WsHeartbeatInterval"`
	HttpJsonPort        int              `json:"HttpJsonPort"`
	NodePort            int              `json:"NodePort"`
	NodeType            string           `json:"NodeType"`
	WebSocketPort       int              `json:"WebSocketPort"`
	PrintLevel          int              `json:"PrintLevel"`
	IsTLS               bool             `json:"IsTLS"`
	CertPath            string           `json:"CertPath"`
	KeyPath             string           `json:"KeyPath"`
	CAPath              string           `json:"CAPath"`
	GenBlockTime        uint             `json:"GenBlockTime"`
	MultiCoreNum        uint             `json:"MultiCoreNum"`
	EncryptAlg          string           `json:"EncryptAlg"`
	MaxLogSize          int64            `json:"MaxLogSize"`
	MaxTxInBlock        int              `json:"MaxTransactionInBlock"`
	MaxBlockSize        int              `json:"MaxBlockSize"`
	PowConfiguration    PowConfiguration `json:"PowConfiguration"`
	MaxHdrSyncReqs      int              `json:"MaxConcurrentSyncHeaderReqs"`
	DefaultMaxPeers     uint             `json:"DefaultMaxPeers"`
	GetAddrMax          uint             `json:"GetAddrMax"`
	MaxOutboundCnt      uint             `json:"MaxOutboundCnt"`
	AddCheckpoints []string `json:"AddCheckpoints"`
}

type ConfigFile struct {
	ConfigFile Configuration `json:"Configuration"`
}

type ChainParams struct {
	Name               string
	PowLimit           *big.Int
	PowLimitBits       uint32
	TargetTimespan     time.Duration
	TargetTimePerBlock time.Duration
	AdjustmentFactor   int64
	MaxOrphanBlocks    int
	MinMemoryNodes     uint32
	SpendCoinbaseSpan  uint32
}

type configParams struct {
	*Configuration
	*ChainParams
}

var Parameters configParams

func init() {
	file, e := ioutil.ReadFile(DefaultConfigFilename)
	if e != nil {
		log.Fatalf("File error: %v\n", e)
		os.Exit(1)
	}
	// Remove the UTF-8 Byte Order Mark
	file = bytes.TrimPrefix(file, []byte("\xef\xbb\xbf"))

	config := ConfigFile{}
	e = json.Unmarshal(file, &config)
	if e != nil {
		log.Fatalf("Unmarshal json file erro %v", e)
		os.Exit(1)
	}
	Parameters.Configuration = &(config.ConfigFile)
}
