package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/nknorg/go-portscanner"
	"github.com/rdegges/go-ipify"
)

const (
	DefaultConfigFilename = "./config.json"
)

const (
	ConsensusTime      = 10 * time.Second
	ProposerChangeTime = time.Minute
)

var (
	Version       string
	SkipCheckPort bool
	Parameters    = &Configuration{
		Magic:         99281,
		Version:       1,
		ChordPort:     30000,
		NodePort:      30001,
		HttpWsPort:    30002,
		HttpJsonPort:  30003,
		LogLevel:      1,
		ConsensusType: "ising",
		SeedList: []string{
			"http://127.0.0.1:30003",
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
	GenesisBlockProposer string   `json:"GenesisBlockProposer"`
	Hostname             string   `json:"Hostname"`
}

func Init() error {
	if _, err := os.Stat(DefaultConfigFilename); err == nil {
		file, err := ioutil.ReadFile(DefaultConfigFilename)
		if err != nil {
			return err
		}

		// Remove the UTF-8 Byte Order Mark
		file = bytes.TrimPrefix(file, []byte("\xef\xbb\xbf"))

		err = json.Unmarshal(file, Parameters)
		if err != nil {
			return err
		}
	} else {
		log.Println("Config file not exists, use default parameters.")
	}

	Parameters.IncrementPort()

	if Parameters.Hostname == "" {
		ip, err := ipify.GetIp()
		if err != nil {
			return err
		}

		Parameters.Hostname = ip

		if !SkipCheckPort {
			ok, err := Parameters.CheckPorts(ip)
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("Some ports are not open. Please make sure you set up port forwarding or firewall correctly")
			}
		}
	}

	err := check(Parameters)
	if err != nil {
		return err
	}

	return nil
}

func check(config *Configuration) error {
	switch config.ConsensusType {
	case "ising":
		if len(config.SeedList) == 0 {
			return errors.New("seed list in config file should not be blank")
		}
	default:
		return fmt.Errorf("invalid consensus type %s in config file", config.ConsensusType)
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
	if delta > 0 {
		log.Println("[WARNING] Port in use! All ports are automatically increased by", delta)
	}
}

func checkPort(host string, port uint16) (bool, error) {
	conn, err := net.Listen("tcp", ":"+strconv.Itoa(int(port)))
	if err != nil {
		return false, fmt.Errorf("Port %d is in use", port)
	}
	defer conn.Close()

	isOpen, err := portscanner.CheckTCP(host, port)
	return isOpen, err
}

func (config *Configuration) CheckPorts(myIP string) (bool, error) {
	allPorts := []uint16{
		config.ChordPort,
		config.NodePort,
		config.HttpWsPort,
		config.HttpJsonPort,
	}
	for _, port := range allPorts {
		log.Printf("[INFO] Checking TCP port %d", port)
		isOpen, err := checkPort(myIP, port)
		if err != nil {
			return false, err
		}
		if !isOpen {
			return false, fmt.Errorf("Port %d is not open", port)
		}
		log.Printf("[INFO] Port %d is open", port)
	}
	return true, nil
}
