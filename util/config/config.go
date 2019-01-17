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
	"github.com/nknorg/nnet/transport"
	"github.com/rdegges/go-ipify"
)

const (
	DefaultConfigFilename = "./config.json"
)

const (
	ConsensusTime       = 18 * time.Second
	ProposerChangeTime  = time.Minute
	DefaultMiningReward = 15
	MinNumSuccessors    = 8
	NodeIDBytes         = 32
)

var (
	Version       string
	SkipCheckPort bool
	SkipNAT       bool
	Parameters    = &Configuration{
		Version:       1,
		Transport:     "tcp",
		NodePort:      30001,
		HttpWsPort:    30002,
		HttpJsonPort:  30003,
		HttpProxyPort: 30004,
		NAT:           true,
		LogLevel:      1,
		SeedList: []string{
			"http://127.0.0.1:30003",
		},
		SyncBatchWindowSize:       1024,
		SyncBlockHeadersBatchSize: 256,
		SyncBlocksBatchSize:       8,
	}
)

type Configuration struct {
	Version                   int      `json:"Version"`
	SeedList                  []string `json:"SeedList"`
	RestCertPath              string   `json:"RestCertPath"`
	RestKeyPath               string   `json:"RestKeyPath"`
	RPCCert                   string   `json:"RPCCert"`
	RPCKey                    string   `json:"RPCKey"`
	HttpWsPort                uint16   `json:"HttpWsPort"`
	HttpJsonPort              uint16   `json:"HttpJsonPort"`
	HttpProxyPort             uint16   `json:"HttpProxyPort"`
	HttpProxyDialTimeout      uint16   `json:"HttpProxyDialTimeout"`
	NodePort                  uint16   `json:"-"`
	LogLevel                  int      `json:"LogLevel"`
	IsTLS                     bool     `json:"IsTLS"`
	CertPath                  string   `json:"CertPath"`
	KeyPath                   string   `json:"KeyPath"`
	CAPath                    string   `json:"CAPath"`
	GenBlockTime              uint     `json:"GenBlockTime"`
	EncryptAlg                string   `json:"EncryptAlg"`
	MaxLogSize                int64    `json:"MaxLogSize"`
	MaxTxInBlock              int      `json:"MaxTransactionInBlock"`
	MaxHdrSyncReqs            int      `json:"MaxConcurrentSyncHeaderReqs"`
	GenesisBlockProposer      string   `json:"GenesisBlockProposer"`
	Hostname                  string   `json:"Hostname"`
	Transport                 string   `json:"Transport"`
	NAT                       bool     `json:"NAT"`
	BeneficiaryAddr           string   `json:"BeneficiaryAddr"`
	SyncBatchWindowSize       uint32   `json:"SyncBatchWindowSize"`
	SyncBlockHeadersBatchSize uint32   `json:"SyncBlockHeadersBatchSize"`
	SyncBlocksBatchSize       uint32   `json:"SyncBlocksBatchSize"`
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

	if Parameters.NAT && !SkipNAT {
		log.Println("Discovering NAT gateway...")

		nat, err := gonat.DiscoverGateway()
		if err == nil {
			log.Printf("Found %s gateway", nat.Type())

			err = Parameters.addPortMapping(nat)
			if err != nil {
				log.Printf("Error adding port mapping. If this problem persists, you can use --no-nat flag to bypass automatic port forwarding and set it up yourself.")
				return err
			}
		} else {
			log.Printf("No NAT gateway detected, skip automatic port forwading. You need to set up port forwarding and firewall yourself.")
		}
	} else {
		log.Printf("Skip automatic port forwading. You need to set up port forwarding and firewall yourself.")
	}

	if Parameters.Hostname == "" {
		ip, err := ipify.GetIp()
		if err != nil {
			return err
		}

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

func (config *Configuration) addPortMapping(nat gonat.NAT) error {
	transport, err := transport.NewTransport(config.Transport)
	if err != nil {
		return err
	}

	externalPort, internalPort, err := nat.AddPortMapping(transport.GetNetwork(), int(config.NodePort), int(config.NodePort), "nkn", 10*time.Second)
	if err != nil {
		return err
	}
	log.Printf("Mapped external port %d to internal port %d", externalPort, internalPort)

	externalPort, internalPort, err = nat.AddPortMapping(transport.GetNetwork(), int(config.HttpWsPort), int(config.HttpWsPort), "nkn", 10*time.Second)
	if err != nil {
		return err
	}
	log.Printf("Mapped external port %d to internal port %d", externalPort, internalPort)

	externalPort, internalPort, err = nat.AddPortMapping(transport.GetNetwork(), int(config.HttpJsonPort), int(config.HttpJsonPort), "nkn", 10*time.Second)
	if err != nil {
		return err
	}
	log.Printf("Mapped external port %d to internal port %d", externalPort, internalPort)

	externalPort, internalPort, err = nat.AddPortMapping(transport.GetNetwork(), int(config.HttpProxyPort), int(config.HttpProxyPort), "nkn", 10*time.Second)
	if err != nil {
		return err
	}
	log.Printf("Mapped external port %d to internal port %d", externalPort, internalPort)

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
		config.HttpProxyPort,
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
	config.HttpProxyPort += delta
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
		config.HttpProxyPort,
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
