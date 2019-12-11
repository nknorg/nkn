package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	gonat "github.com/nknorg/go-nat"
	portscanner "github.com/nknorg/go-portscanner"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto/ed25519"
	"github.com/nknorg/nkn/crypto/ed25519/vrf"
	"github.com/nknorg/nkn/crypto/util"
	"github.com/nknorg/nnet/transport"
	"github.com/pbnjay/memory"
	goupnp "gitlab.com/NebulousLabs/go-upnp"
)

const (
	MaxNumTxnPerBlock            = 4096
	MaxBlockSize                 = 1 * 1024 * 1024 // in bytes
	ConsensusDuration            = 20 * time.Second
	ConsensusTimeout             = 60 * time.Second
	MinNumSuccessors             = 8
	NodeIDBytes                  = 32
	MaxRollbackBlocks            = 180
	SigChainBlockDelay           = 1
	SigChainPropogationTime      = 2
	GossipSampleChordNeighbor    = 0.1
	GossipSampleRandomNeighbor   = 1.0
	VotingSampleChordNeighbor    = 0.5
	VotingSampleRandomNeighbor   = 0.0
	HeaderVersion                = 1
	DBVersion                    = 0x01
	InitialIssueAddress          = "NKNFCrUMFPkSeDRMG2ME21hD6wBCA2poc347"
	InitialIssueAmount           = 700000000 * common.StorageFactor
	TotalMiningRewards           = 300000000 * common.StorageFactor
	TotalRewardDuration          = uint32(25)
	InitialReward                = common.Fixed64(18000000 * common.StorageFactor)
	RewardAdjustInterval         = 365 * 24 * 60 * 60 / int(ConsensusDuration/time.Second)
	ReductionAmount              = common.Fixed64(500000 * common.StorageFactor)
	DonationAddress              = "NKNaaaaaaaaaaaaaaaaaaaaaaaaaaaeJ6gxa"
	DonationAdjustDividendFactor = 1
	DonationAdjustDivisorFactor  = 2
	MinGenIDRegistrationFee      = 0
	GenerateIDBlockDelay         = 8
	RandomBeaconUniqueLength     = vrf.Size
	RandomBeaconLength           = vrf.Size + vrf.ProofSize
	ProtocolVersion              = 30
	MinCompatibleProtocolVersion = 30
	MaxCompatibleProtocolVersion = 39
	TxPoolCleanupInterval        = ConsensusDuration
	ShortHashSize                = uint32(8)
	MaxAssetPrecision            = uint32(8)
	NKNAssetName                 = "NKN"
	NKNAssetSymbol               = "nkn"
	NKNAssetPrecision            = uint32(8)
	GASAssetName                 = "New Network Coin"
	GASAssetSymbol               = "nnc"
	GASAssetPrecision            = uint32(8)
	DumpMemInterval              = 30 * time.Second
	MaxClientMessageSize         = 4 * 1024 * 1024
)

const (
	defaultConfigFile             = "config.json"
	defaultSyncMaxMemoryPercent   = 25
	defaultSyncBatchWindowSize    = 64
	defaultTxPoolMaxMemoryPercent = 0.4
	defaultTxPoolMaxMemorySize    = 32
)

var (
	Debug            = false
	PprofPort        = "127.0.0.1:8080"
	ShortHashSalt    = util.RandomBytes(32)
	GenesisTimestamp = time.Date(2019, time.June, 29, 13, 10, 13, 0, time.UTC).Unix()
	GenesisBeacon    = make([]byte, RandomBeaconLength)
	NKNAssetID       = common.Uint256{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	GASAssetID = common.Uint256{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
	}
	MaxSubscribeIdentifierLen = HeightDependentInt32{
		heights: []uint32{133400, 0},
		values:  []int32{64, math.MaxInt32},
	}
	MaxSubscribeMetaLen = HeightDependentInt32{
		heights: []uint32{133400, 0},
		values:  []int32{1024, math.MaxInt32},
	}
	MaxSubscribeBucket = HeightDependentInt32{
		heights: []uint32{245000, 0},
		values:  []int32{0, 1000},
	}
	MaxSubscribeDuration = HeightDependentInt32{
		heights: []uint32{245000, 0},
		values:  []int32{400000, 65535},
	}
	MaxSubscriptionsCount = 100000
	MaxGenerateIDTxnHash  = HeightDependentUint256{
		heights: []uint32{245000, 0},
		values: []common.Uint256{
			common.Uint256{
				0x00, 0x00, 0x00, 0x07, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			},
			common.MaxUint256,
		},
	}
	MaxTxnAttributesLen  = 100
	AllowTxnRegisterName = HeightDependentBool{
		heights: []uint32{7500, 0},
		values:  []bool{false, true},
	}
)

var (
	Version                      string
	SkipNAT                      bool
	gateway                      interface{}
	ConfigFile                   string
	LogPath                      string
	ChainDBPath                  string
	WalletFile                   string
	BeneficiaryAddr              string
	SeedList                     string
	GenesisBlockProposer         string
	AllowEmptyBeneficiaryAddress bool
	WebGuiListenAddress          string
	WebGuiCreateWallet           bool
	PasswordFile                 string
	StatePruningMode             string
	Parameters                   = &Configuration{
		Version:                      1,
		Transport:                    "tcp",
		NodePort:                     30001,
		HttpWsPort:                   30002,
		HttpJsonPort:                 30003,
		NAT:                          true,
		Mining:                       true,
		MiningDebug:                  true,
		LogLevel:                     1,
		MaxLogFileSize:               20,
		SyncBatchWindowSize:          0,
		SyncBlockHeadersBatchSize:    128,
		SyncBlocksBatchSize:          4,
		SyncBlocksMaxMemorySize:      0,
		RPCReadTimeout:               5,
		RPCWriteTimeout:              10,
		RPCIdleTimeout:               0,
		RPCKeepAlivesEnabled:         false,
		NATPortMappingTimeout:        365 * 86400,
		NumTxnPerBlock:               256,
		TxPoolPerAccountTxCap:        32,
		TxPoolTotalTxCap:             0,
		TxPoolMaxMemorySize:          0,
		RegisterIDRegFee:             0,
		RegisterIDTxnFee:             0,
		LogPath:                      "Log",
		ChainDBPath:                  "ChainDB",
		WalletFile:                   "wallet.json",
		MaxGetIDSeeds:                3,
		DBFilesCacheCapacity:         100,
		NumLowFeeTxnPerBlock:         0,
		LowFeeTxnSizePerBlock:        2048,
		MinTxnFee:                    10000000,
		AllowEmptyBeneficiaryAddress: false,
		WebGuiListenAddress:          "127.0.0.1",
		WebGuiPort:                   30000,
		WebGuiCreateWallet:           false,
		PasswordFile:                 "",
		RecentStateCount:             1024,
		StatePruningMode:             "none",
	}
)

type Configuration struct {
	Version                      int           `json:"Version"`
	SeedList                     []string      `json:"SeedList"`
	RestCertPath                 string        `json:"RestCertPath"`
	RestKeyPath                  string        `json:"RestKeyPath"`
	RPCCert                      string        `json:"RPCCert"`
	RPCKey                       string        `json:"RPCKey"`
	HttpWsPort                   uint16        `json:"HttpWsPort"`
	HttpJsonPort                 uint16        `json:"HttpJsonPort"`
	NodePort                     uint16        `json:"-"`
	LogLevel                     int           `json:"LogLevel"`
	MaxLogFileSize               uint32        `json:"MaxLogSize"`
	IsTLS                        bool          `json:"IsTLS"`
	CertPath                     string        `json:"CertPath"`
	KeyPath                      string        `json:"KeyPath"`
	CAPath                       string        `json:"CAPath"`
	GenesisBlockProposer         string        `json:"GenesisBlockProposer"`
	NumLowFeeTxnPerBlock         uint32        `json:"NumLowFeeTxnPerBlock"`
	LowFeeTxnSizePerBlock        uint32        `json:"LowFeeTxnSizePerBlock"` // in bytes
	MinTxnFee                    int64         `json:"MinTxnFee"`
	RegisterIDRegFee             int64         `json:"RegisterIDRegFee"`
	RegisterIDTxnFee             int64         `json:"RegisterIDTxnFee"`
	Hostname                     string        `json:"Hostname"`
	Transport                    string        `json:"Transport"`
	NAT                          bool          `json:"NAT"`
	Mining                       bool          `json:"Mining"`
	MiningDebug                  bool          `json:"MiningDebug"`
	BeneficiaryAddr              string        `json:"BeneficiaryAddr"`
	SyncBatchWindowSize          uint32        `json:"SyncBatchWindowSize"`
	SyncBlockHeadersBatchSize    uint32        `json:"SyncBlockHeadersBatchSize"`
	SyncBlocksBatchSize          uint32        `json:"SyncBlocksBatchSize"`
	SyncBlocksMaxMemorySize      uint32        `json:"SyncBlocksMaxMemorySize"` // in megabytes (MB)
	NumTxnPerBlock               uint32        `json:"NumTxnPerBlock"`
	TxPoolPerAccountTxCap        uint32        `json:"TxPoolPerAccountTxCap"`
	TxPoolTotalTxCap             uint32        `json:"TxPoolTotalTxCap"`
	TxPoolMaxMemorySize          uint32        `json:"TxPoolMaxMemorySize"` // in megabytes (MB)
	RPCReadTimeout               time.Duration `json:"RPCReadTimeout"`      // in seconds
	RPCWriteTimeout              time.Duration `json:"RPCWriteTimeout"`     // in seconds
	RPCIdleTimeout               time.Duration `json:"RPCIdleTimeout"`      // in seconds
	RPCKeepAlivesEnabled         bool          `json:"RPCKeepAlivesEnabled"`
	NATPortMappingTimeout        time.Duration `json:"NATPortMappingTimeout"` // in seconds
	LogPath                      string        `json:"LogPath"`
	ChainDBPath                  string        `json:"ChainDBPath"`
	WalletFile                   string        `json:"WalletFile"`
	MaxGetIDSeeds                uint32        `json:"MaxGetIDSeeds"`
	DBFilesCacheCapacity         uint32        `json:"DBFilesCacheCapacity"`
	AllowEmptyBeneficiaryAddress bool          `json:"AllowEmptyBeneficiaryAddress"`
	WebGuiListenAddress          string        `json:"WebGuiListenAddress"`
	WebGuiPort                   uint16        `json:"WebGuiPort"`
	WebGuiCreateWallet           bool          `json:"WebGuiCreateWallet"`
	PasswordFile                 string        `json:"PasswordFile"`
	RecentStateCount             uint32        `json:"RecentStateCount"`
	StatePruningMode             string        `json:"StatePruningMode"`
}

func Init() error {
	file, err := OpenConfigFile()
	if err == nil {
		err = json.Unmarshal(file, Parameters)
		if err != nil {
			return err
		}
	} else {
		log.Println("Config file not exists, use default parameters.")
	}

	if len(LogPath) > 0 {
		Parameters.LogPath = LogPath
	}

	if len(ChainDBPath) > 0 {
		Parameters.ChainDBPath = ChainDBPath
	}

	if len(WalletFile) > 0 {
		Parameters.WalletFile = WalletFile
	}

	if len(BeneficiaryAddr) > 0 {
		Parameters.BeneficiaryAddr = BeneficiaryAddr
	}

	if len(SeedList) > 0 {
		Parameters.SeedList = strings.Split(SeedList, ",")
	}

	if len(GenesisBlockProposer) > 0 {
		Parameters.GenesisBlockProposer = GenesisBlockProposer
	}

	if Parameters.Hostname == "127.0.0.1" {
		Parameters.incrementPort()
	}

	if AllowEmptyBeneficiaryAddress {
		Parameters.AllowEmptyBeneficiaryAddress = AllowEmptyBeneficiaryAddress
	}

	if len(WebGuiListenAddress) > 0 {
		Parameters.WebGuiListenAddress = WebGuiListenAddress
	}

	if WebGuiCreateWallet {
		Parameters.WebGuiCreateWallet = WebGuiCreateWallet
	}

	if len(PasswordFile) > 0 {
		Parameters.PasswordFile = PasswordFile
	}

	if len(StatePruningMode) > 0 {
		Parameters.StatePruningMode = StatePruningMode
	}

	if Parameters.SyncBatchWindowSize == 0 {
		syncBlocksMaxMemorySize := uint64(Parameters.SyncBlocksMaxMemorySize) * 1024 * 1024
		if syncBlocksMaxMemorySize == 0 {
			syncBlocksMaxMemorySize = uint64(float64(memory.TotalMemory()) * defaultSyncMaxMemoryPercent / 100.0)
		}

		Parameters.SyncBatchWindowSize = uint32(syncBlocksMaxMemorySize/MaxBlockSize) / Parameters.SyncBlocksBatchSize
		if Parameters.SyncBatchWindowSize == 0 {
			Parameters.SyncBatchWindowSize = defaultSyncBatchWindowSize
		}
		log.Printf("Set SyncBatchWindowSize to %v", Parameters.SyncBatchWindowSize)
	}

	if Parameters.TxPoolMaxMemorySize == 0 {
		Parameters.TxPoolMaxMemorySize = uint32(float64(memory.TotalMemory()) / 1024 / 1024 * defaultTxPoolMaxMemoryPercent / 100.0)
		if Parameters.TxPoolMaxMemorySize == 0 {
			Parameters.TxPoolMaxMemorySize = defaultTxPoolMaxMemorySize
		}
		log.Printf("Set TxPoolMaxMemorySize to %v", Parameters.TxPoolMaxMemorySize)
	}

	err = Parameters.verify()
	if err != nil {
		return err
	}

	if err := Parameters.SetupPortMapping(); err != nil {
		log.Printf("Error adding port mapping. If this problem persists, you can use --no-nat flag to bypass automatic port forwarding and set it up yourself.")
		return err
	}

	return nil
}

func (config *Configuration) SetupPortMapping() error {
	if SkipNAT || !config.NAT {
		log.Printf("Skip automatic port forwading. You need to set up port forwarding and firewall yourself.")
		return nil
	}

	log.Println("Discovering NAT gateway...")

	upnp, err := goupnp.Discover()
	if err == nil {
		log.Printf("Discovered UPnP gateway")

		gateway = upnp

		err = upnp.Forward(config.NodePort, "NKN Node")
		if err != nil {
			return err
		}
		log.Printf("Mapped external port %d to internal port %d", config.NodePort, config.NodePort)

		err = upnp.Forward(config.HttpWsPort, "NKN Node")
		if err != nil {
			return err
		}
		log.Printf("Mapped external port %d to internal port %d", config.HttpWsPort, config.HttpWsPort)

		err = upnp.Forward(config.HttpJsonPort, "NKN Node")
		if err != nil {
			return err
		}
		log.Printf("Mapped external port %d to internal port %d", config.HttpJsonPort, config.HttpJsonPort)
	} else {
		log.Printf("No UPnP gateway discovered, trying go-nat...")

		nat, err := gonat.DiscoverGateway()
		if err != nil {
			log.Printf("No NAT gateway discovered, skip automatic port forwading. You need to set up port forwarding and firewall yourself.")
			return nil
		}

		log.Printf("Found %s gateway", nat.Type())

		gateway = nat

		transport, err := transport.NewTransport(config.Transport)
		if err != nil {
			return err
		}

		externalPort, internalPort, err := nat.AddPortMapping(transport.GetNetwork(), int(config.NodePort), int(config.NodePort), "NKN Node", config.NATPortMappingTimeout*time.Second)
		if err != nil {
			return err
		}
		log.Printf("Mapped external port %d to internal port %d", externalPort, internalPort)

		externalPort, internalPort, err = nat.AddPortMapping(transport.GetNetwork(), int(config.HttpWsPort), int(config.HttpWsPort), "NKN Node", config.NATPortMappingTimeout*time.Second)
		if err != nil {
			return err
		}
		log.Printf("Mapped external port %d to internal port %d", externalPort, internalPort)

		externalPort, internalPort, err = nat.AddPortMapping(transport.GetNetwork(), int(config.HttpJsonPort), int(config.HttpJsonPort), "NKN Node", config.NATPortMappingTimeout*time.Second)
		if err != nil {
			return err
		}
		log.Printf("Mapped external port %d to internal port %d", externalPort, internalPort)
	}

	return nil
}

func (config *Configuration) ClearPortMapping() error {
	if SkipNAT || !config.NAT {
		return nil
	}

	if gateway == nil {
		return fmt.Errorf("NAT gateway has not been discovered yet")
	}

	log.Println("Removing added port mapping...")

	switch device := gateway.(type) {
	case *goupnp.IGD:
		err := device.Clear(config.NodePort)
		if err != nil {
			return err
		}
		log.Printf("Removed port mapping at external port %d", config.NodePort)

		err = device.Clear(config.HttpWsPort)
		if err != nil {
			return err
		}
		log.Printf("Removed port mapping at external port %d", config.HttpWsPort)

		err = device.Clear(config.HttpJsonPort)
		if err != nil {
			return err
		}
		log.Printf("Removed port mapping at external port %d", config.HttpJsonPort)
	case gonat.NAT:
		transport, err := transport.NewTransport(config.Transport)
		if err != nil {
			return err
		}

		err = device.DeletePortMapping(transport.GetNetwork(), int(config.NodePort))
		if err != nil {
			return err
		}
		log.Printf("Removed port mapping at external port %d", config.NodePort)

		err = device.DeletePortMapping(transport.GetNetwork(), int(config.HttpWsPort))
		if err != nil {
			return err
		}
		log.Printf("Removed port mapping at external port %d", config.HttpWsPort)

		err = device.DeletePortMapping(transport.GetNetwork(), int(config.HttpJsonPort))
		if err != nil {
			return err
		}
		log.Printf("Removed port mapping at external port %d", config.HttpJsonPort)
	default:
		return fmt.Errorf("Unknown NAT gateway type")
	}

	return nil
}

func (config *Configuration) verify() error {
	if len(config.SeedList) == 0 {
		return errors.New("seed list in config file should not be blank")
	}

	if config.NumTxnPerBlock > MaxNumTxnPerBlock {
		return fmt.Errorf("NumTxnPerBlock cannot be greater than %d", MaxNumTxnPerBlock)
	}

	if len(config.GenesisBlockProposer) == 0 {
		return errors.New("no GenesisBlockProposer")
	}

	pk, err := common.HexStringToBytes(config.GenesisBlockProposer)
	if err != nil {
		return fmt.Errorf("parse GenesisBlockProposer error: %v", err)
	}
	if len(pk) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid GenesisBlockProposer length %d bytes, expecting %d bytes", len(pk), ed25519.PublicKeySize)
	}

	if len(config.BeneficiaryAddr) > 0 {
		_, err = common.ToScriptHash(config.BeneficiaryAddr)
		if err != nil {
			return fmt.Errorf("parse BeneficiaryAddr error: %v", err)
		}
	}

	if config.MaxLogFileSize <= 0 {
		return fmt.Errorf("MaxLogFileSize should be >= 1 (MB)")
	}

	switch config.StatePruningMode {
	case "lowmem":
	case "none":
	default:
		return fmt.Errorf("unknown state pruning mode %v", config.StatePruningMode)
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

func GetConfigFile() string {
	configFile := ConfigFile
	if configFile == "" {
		configFile = defaultConfigFile
	}
	return configFile
}

func OpenConfigFile() ([]byte, error) {
	var file []byte

	configFile := GetConfigFile()
	_, err := os.Stat(configFile)
	if err != nil {
		return nil, err
	}
	file, err = ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	// Remove the UTF-8 Byte Order Mark
	file = bytes.TrimPrefix(file, []byte("\xef\xbb\xbf"))
	return file, nil

}

func WriteConfigFile(configuration map[string]interface{}) error {
	bytes, err := json.MarshalIndent(&configuration, "", "    ")
	if err != nil {
		return err
	}
	configFile := GetConfigFile()
	return ioutil.WriteFile(configFile, bytes, 0666)
}

func SetBeneficiaryAddr(addr string, allowEmpty bool) error {
	if !allowEmpty {
		if addr == "" {
			return errors.New("beneficiary address is empty.")
		}
	}

	if addr != "" {
		_, err := common.ToScriptHash(addr)
		if err != nil {
			return errors.New(fmt.Sprintf("parse BeneficiaryAddr error: %v", err))
		}
	}

	file, err := OpenConfigFile()
	if err != nil {
		return err
	}
	var configuration map[string]interface{}
	err = json.Unmarshal(file, &configuration)
	if err != nil {
		return err
	}

	// set beneficiary address
	configuration["BeneficiaryAddr"] = addr

	err = WriteConfigFile(configuration)
	if err != nil {
		return err
	}
	Parameters.BeneficiaryAddr = addr
	return nil
}
