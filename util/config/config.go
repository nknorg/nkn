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

	gonat "github.com/nknorg/go-nat"
	"github.com/nknorg/go-portscanner"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nnet/transport"
	"github.com/rdegges/go-ipify"
)

const (
	DefaultConfigFilename = "./config.json"
)

const (
	MaxNumTxnPerBlock            = 4096
	MaxBlockSize                 = 1 * 1024 * 1024 // The Max of block size is 1 MB.
	ConsensusDuration            = 20 * time.Second
	ConsensusTimeout             = time.Minute
	DefaultMiningReward          = 15
	MinNumSuccessors             = 8
	NodeIDBytes                  = 32
	MaxRollbackBlocks            = 1
	EncryptAlg                   = "Ed25519"
	DBVersion                    = 0x04
	InitialIssueAddress          = "NKNQ83xc8zQNEE6WBDKm7tZrLwoMwAq4c4jo"
	InitialIssueAmount           = 700000000 * common.StorageFactor
	TotalMiningRewards           = 300000000 * common.StorageFactor
	TotalRewardDuration          = uint32(25)
	InitialReward                = common.Fixed64(18000000 * common.StorageFactor)
	RewardAdjustInterval         = 365 * 24 * 60 * 60 / int(ConsensusDuration/time.Second)
	ReductionAmount              = common.Fixed64(500000 * common.StorageFactor)
	DonationAddress              = "NKNaaaaaaaaaaaaaaaaaaaaaaaaaaaeJ6gxa"
	DonationAdjustDividendFactor = 1
	DonationAdjustDivisorFactor  = 2
)

var (
	Version       string
	SkipCheckPort bool
	SkipNAT       bool
	nat           gonat.NAT
	Parameters    = &Configuration{
		Version:      1,
		Transport:    "tcp",
		NodePort:     30001,
		HttpWsPort:   30002,
		HttpJsonPort: 30003,
		NAT:          true,
		MiningDebug:  true,
		LogLevel:     1,
		SeedList: []string{
			"http://127.0.0.1:30003",
		},
		SyncBatchWindowSize:       1024,
		SyncBlockHeadersBatchSize: 256,
		SyncBlocksBatchSize:       8,
		RPCReadTimeout:            5,
		RPCWriteTimeout:           10,
		KeepAliveTimeout:          15,
		NATPortMappingTimeout:     365 * 86400,
	}
)

type Configuration struct {
	Version                   int           `json:"Version"`
	SeedList                  []string      `json:"SeedList"`
	RestCertPath              string        `json:"RestCertPath"`
	RestKeyPath               string        `json:"RestKeyPath"`
	RPCCert                   string        `json:"RPCCert"`
	RPCKey                    string        `json:"RPCKey"`
	HttpWsPort                uint16        `json:"HttpWsPort"`
	HttpJsonPort              uint16        `json:"HttpJsonPort"`
	NodePort                  uint16        `json:"-"`
	LogLevel                  int           `json:"LogLevel"`
	IsTLS                     bool          `json:"IsTLS"`
	CertPath                  string        `json:"CertPath"`
	KeyPath                   string        `json:"KeyPath"`
	CAPath                    string        `json:"CAPath"`
	GenBlockTime              uint          `json:"GenBlockTime"`
	MaxLogSize                int64         `json:"MaxLogSize"`
	MaxHdrSyncReqs            int           `json:"MaxConcurrentSyncHeaderReqs"`
	GenesisBlockProposer      string        `json:"GenesisBlockProposer"`
	MinTxnFee                 int64         `json:"MinTxnFee"`
	Hostname                  string        `json:"Hostname"`
	Transport                 string        `json:"Transport"`
	NAT                       bool          `json:"NAT"`
	MiningDebug               bool          `json:"MiningDebug"`
	BeneficiaryAddr           string        `json:"BeneficiaryAddr"`
	SyncBatchWindowSize       uint32        `json:"SyncBatchWindowSize"`
	SyncBlockHeadersBatchSize uint32        `json:"SyncBlockHeadersBatchSize"`
	SyncBlocksBatchSize       uint32        `json:"SyncBlocksBatchSize"`
	RPCReadTimeout            time.Duration `json:"RPCReadTimeout"`        // in seconds
	RPCWriteTimeout           time.Duration `json:"RPCWriteTimeout"`       // in seconds
	KeepAliveTimeout          time.Duration `json:"KeepAliveTimeout"`      // in seconds
	NATPortMappingTimeout     time.Duration `json:"NATPortMappingTimeout"` // in seconds
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

	if Parameters.Hostname == "127.0.0.1" {
		Parameters.incrementPort()
	}

	if err := Parameters.SetupPortMapping(); err != nil {
		log.Printf("Error adding port mapping. If this problem persists, you can use --no-nat flag to bypass automatic port forwarding and set it up yourself.")
		return err
	}

	if Parameters.Hostname == "" {
		log.Println("Getting my IP address...")
		ip, err := ipify.GetIp()
		if err != nil {
			return err
		}
		log.Printf("My IP address is %s", ip)

		Parameters.Hostname = ip

		// if !SkipCheckPort {
		// 	ok, err := Parameters.CheckPorts(ip)
		// 	if err != nil {
		// 		return err
		// 	}
		// 	if !ok {
		// 		return errors.New("Some ports are not open. Please make sure you set up port forwarding or firewall correctly")
		// 	}
		// }
	}

	err := check(Parameters)
	if err != nil {
		return err
	}

	return nil
}

func (config *Configuration) SetupPortMapping() error {
	if config.NAT && !SkipNAT {
		log.Println("Discovering NAT gateway...")

		var err error
		nat, err = gonat.DiscoverGateway()
		if err != nil {
			log.Printf("No NAT gateway detected, skip automatic port forwading. You need to set up port forwarding and firewall yourself.")
			return nil
		}

		log.Printf("Found %s gateway", nat.Type())

		transport, err := transport.NewTransport(config.Transport)
		if err != nil {
			return err
		}

		externalPort, internalPort, err := nat.AddPortMapping(transport.GetNetwork(), int(config.NodePort), int(config.NodePort), "nkn", config.NATPortMappingTimeout*time.Second)
		if err != nil {
			return err
		}
		log.Printf("Mapped external port %d to internal port %d", externalPort, internalPort)

		externalPort, internalPort, err = nat.AddPortMapping(transport.GetNetwork(), int(config.HttpWsPort), int(config.HttpWsPort), "nkn", config.NATPortMappingTimeout*time.Second)
		if err != nil {
			return err
		}
		log.Printf("Mapped external port %d to internal port %d", externalPort, internalPort)

		externalPort, internalPort, err = nat.AddPortMapping(transport.GetNetwork(), int(config.HttpJsonPort), int(config.HttpJsonPort), "nkn", config.NATPortMappingTimeout*time.Second)
		if err != nil {
			return err
		}
		log.Printf("Mapped external port %d to internal port %d", externalPort, internalPort)
	} else {
		log.Printf("Skip automatic port forwading. You need to set up port forwarding and firewall yourself.")
	}

	return nil
}

func (config *Configuration) CleanPortMapping() error {
	if config.NAT && !SkipNAT {
		if nat == nil {
			return fmt.Errorf("NAT gateway has not been discoverred yet")
		}

		log.Println("Removing added port mapping...")

		transport, err := transport.NewTransport(config.Transport)
		if err != nil {
			return err
		}

		err = nat.DeletePortMapping(transport.GetNetwork(), int(config.NodePort))
		if err != nil {
			return err
		}
		log.Printf("Removed port mapping at external port %d", config.NodePort)

		err = nat.DeletePortMapping(transport.GetNetwork(), int(config.HttpWsPort))
		if err != nil {
			return err
		}
		log.Printf("Removed port mapping at external port %d", config.HttpWsPort)

		err = nat.DeletePortMapping(transport.GetNetwork(), int(config.HttpJsonPort))
		if err != nil {
			return err
		}
		log.Printf("Removed port mapping at external port %d", config.HttpJsonPort)
	}
	return nil
}

func check(config *Configuration) error {
	if len(config.SeedList) == 0 {
		return errors.New("seed list in config file should not be blank")
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

func (config *Configuration) incrementPort() {
	allPorts := []uint16{
		config.NodePort,
		config.HttpWsPort,
		config.HttpJsonPort,
	}
	minPort, maxPort := findMinMaxPort(allPorts)
	step := maxPort - minPort + 1
	var delta uint16
	for {
		tcpConn, err := net.Listen("tcp", ":"+strconv.Itoa(int(config.NodePort+delta)))
		if err != nil {
			fmt.Println(err)
			delta += step
			continue
		}
		tcpConn.Close()

		udpAddr, err := net.ResolveUDPAddr("udp", ":"+strconv.Itoa(int(config.NodePort+delta)))
		if err != nil {
			fmt.Println(err)
			return
		}
		udpConn, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			fmt.Println(err)
			delta += step
			continue
		}
		udpConn.Close()

		break
	}
	config.NodePort += delta
	config.HttpWsPort += delta
	config.HttpJsonPort += delta
	if delta > 0 {
		log.Println("Port in use! All ports are automatically increased by", delta)
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
		config.NodePort,
		config.HttpWsPort,
		config.HttpJsonPort,
	}
	for _, port := range allPorts {
		log.Printf("Checking TCP port %d", port)
		isOpen, err := checkPort(myIP, port)
		if err != nil {
			return false, err
		}
		if !isOpen {
			return false, fmt.Errorf("Port %d is not open", port)
		}
		log.Printf("Port %d is open", port)
	}
	return true, nil
}
