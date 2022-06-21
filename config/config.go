package config

import (
	"bytes"
	"encoding/hex"
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

	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/util"
	"github.com/pbnjay/memory"
)

const (
	PublicKeySize                   = 32
	VRFSize                         = 32
	VRFProofSize                    = 96
	MaxNumTxnPerBlock               = 4096
	MaxBlockSize                    = 1 * 1024 * 1024 // in bytes
	ConsensusDuration               = 20 * time.Second
	ConsensusTimeout                = 60 * time.Second
	NodeIDBytes                     = 32
	SigChainBlockDelay              = 1
	SigChainPropogationTime         = 2
	MinNumSuccessors                = 8
	NumRandomGossipNeighborsFactor  = 1
	NumRandomVotingNeighborsFactor  = 3
	MinNumRandomGossipNeighbors     = 8
	MinNumRandomVotingNeighbors     = 24
	MaxNumInboundRandomNeighbors    = 256
	GossipSampleChordNeighbor       = 0.15
	GossipMinChordNeighbor          = 8
	VotingSampleChordNeighbor       = 0.0
	VotingMinChordNeighbor          = 0
	SigChainObjectionSampleNeighbor = 0.1
	HeaderVersion                   = 1
	DBVersion                       = 0x01
	InitialIssueAddress             = "NKNFCrUMFPkSeDRMG2ME21hD6wBCA2poc347"
	InitialIssueAmount              = 700000000 * common.StorageFactor
	TotalMiningRewards              = 300000000 * common.StorageFactor
	TotalRewardDuration             = uint32(25)
	InitialReward                   = common.Fixed64(18000000 * common.StorageFactor)
	RewardAdjustInterval            = 365 * 24 * 60 * 60 / int(ConsensusDuration/time.Second)
	ReductionAmount                 = common.Fixed64(500000 * common.StorageFactor)
	DonationAddress                 = "NKNaaaaaaaaaaaaaaaaaaaaaaaaaaaeJ6gxa"
	DonationAdjustDividendFactor    = 1
	DonationAdjustDivisorFactor     = 2
	GenerateIDBlockDelay            = 8
	RandomBeaconUniqueLength        = VRFSize
	RandomBeaconLength              = VRFSize + VRFProofSize
	ProtocolVersion                 = 40
	MinCompatibleProtocolVersion    = 40
	MaxCompatibleProtocolVersion    = 49
	TxPoolCleanupInterval           = ConsensusDuration
	ShortHashSize                   = uint32(8)
	MaxAssetPrecision               = uint32(8)
	NKNAssetName                    = "NKN"
	NKNAssetSymbol                  = "nkn"
	NKNAssetPrecision               = uint32(8)
	GASAssetName                    = "New Network Coin"
	GASAssetSymbol                  = "nnc"
	GASAssetPrecision               = uint32(8)
	DumpMemInterval                 = 30 * time.Second
	MaxClientMessageSize            = 4 * 1024 * 1024
	MinNameRegistrationFee          = 10 * common.StorageFactor
	DefaultNameDuration             = 365 * 24 * 60 * 60 / int(ConsensusDuration/time.Second)
)

const (
	defaultConfigFile                 = "config.json"
	estimatedHeaderSize               = 418
	defaultSyncStateMaxThread         = 10
	defaultSyncStateThreadMemory      = 30 * 1024 * 1024
	defaultSyncHeaderMaxMemoryPercent = 2.0
	defaultSyncHeaderMaxSize          = 32768
	defaultSyncMaxMemoryPercent       = 25
	defaultSyncBatchWindowSize        = 64
	defaultTxPoolMaxMemoryPercent     = 0.4
	defaultTxPoolMaxMemorySize        = 32
)

var (
	SyncPruning      = true
	LivePruning      = false
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
	AllowSubscribeTopicRegex = HeightDependentString{
		heights: []uint32{980000, 0},
		values:  []string{"(^[A-Za-z0-9][A-Za-z0-9-_.+]{2,254}$)", "(^[A-Za-z][A-Za-z0-9-_.+]{2,254}$)"},
	}
	AllowNameRegex = HeightDependentString{
		heights: []uint32{980000, 0},
		values:  []string{"(^[A-Za-z0-9][A-Za-z0-9-_+]{5,62}$)", "(^[A-Za-z][A-Za-z0-9-_.+]{2,254}$)"},
	}
	LegacyNameService = HeightDependentBool{
		heights: []uint32{980000, 0},
		values:  []bool{false, true},
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
	MaxSubscriptionsCount = 0 // 0 for unlimited
	MaxGenerateIDTxnHash  = HeightDependentUint256{
		heights: []uint32{2570000, 245000, 0},
		values: []common.Uint256{
			common.MaxUint256,
			{
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
		heights: []uint32{980000, 7500, 0},
		values:  []bool{true, false, true},
	}
	ChargeNanoPayTxnFee = HeightDependentBool{
		heights: []uint32{1072500, 0},
		values:  []bool{true, false},
	}
	AllowSigChainHashSignature = HeightDependentBool{
		heights: []uint32{1200000, 0},
		values:  []bool{false, true},
	}
	SigChainBitShiftMaxLength = HeightDependentInt32{
		heights: []uint32{2651000, 2633000, 2543000, 0},
		values:  []int32{18, 16, 14, 0},
	}
	SigChainVerifyFingerTableRange = HeightDependentBool{
		heights: []uint32{2570000, 0},
		values:  []bool{true, false},
	}
	SigChainVerifySkipNode = HeightDependentBool{
		heights: []uint32{2570120, 2570000, 0},
		values:  []bool{false, true, false},
	}
	SigChainObjection = HeightDependentBool{
		heights: []uint32{2570120, 2570000, 0},
		values:  []bool{false, true, false},
	}
	MinGenIDRegistrationFee = HeightDependentInt64{
		heights: []uint32{2570000, 0},
		values:  []int64{10 * common.StorageFactor, 0},
	}
	AllowGenerateIDSender = HeightDependentBool{
		heights: []uint32{2570000, 0},
		values:  []bool{true, false},
	}
	AllowTxnGenerateIDMinVersion = HeightDependentInt32{
		heights: []uint32{2570000, 0},
		values:  []int32{1, 0},
	}
	AllowTxnGenerateIDMaxVersion = HeightDependentInt32{
		heights: []uint32{2570000, 0},
		values:  []int32{1, 0},
	}
	AllowGetIDMinVersion = HeightDependentInt32{
		heights: []uint32{2600000, 2570000, 0},
		values:  []int32{1, 0, 0},
	}
	AllowGetIDMaxVersion = HeightDependentInt32{
		heights: []uint32{2600000, 2570000, 0},
		values:  []int32{1, 1, 0},
	}
	SigChainBitShiftPerElement = HeightDependentInt32{
		heights: []uint32{2651000, 0},
		values:  []int32{8, 4},
	}
	SigChainRecentMinerBlocks   = 4096
	SigChainRecentMinerBitShift = HeightDependentInt32{
		heights: []uint32{2651000, 2633000, 0},
		values:  []int32{10, 4, 0},
	}
	SigChainSkipMinerBlocks     = 4096
	SigChainSkipMinerMaxAllowed = HeightDependentInt32{
		heights: []uint32{2651000, 0},
		values:  []int32{2, 3},
	}
	SigChainSkipMinerBitShift = HeightDependentInt32{
		heights: []uint32{2633000, 0},
		values:  []int32{10, 0},
	}
	SigChainMinerSalt = HeightDependentBool{
		heights: []uint32{2900000, 0},
		values:  []bool{true, false},
	}
	SigChainMinerWeightBase = HeightDependentInt32{
		heights: []uint32{2900000, 0},
		values:  []int32{2, 1},
	}
	SigChainMinerWeightMaxExponent = HeightDependentInt32{
		heights: []uint32{2900000, 0},
		values:  []int32{4, 0},
	}
	DonationNoDelay = HeightDependentBool{
		heights: []uint32{3030000, 0},
		values:  []bool{true, false},
	}
)

var (
	Version                      string
	SkipNAT                      bool
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
	SyncMode                     string
	Parameters                   = &Configuration{
		Version:                      1,
		Transport:                    "tcp",
		NodePort:                     30001,
		HttpWsPort:                   30002,
		HttpWssPort:                  30004,
		HttpJsonPort:                 30003,
		HttpsJsonPort:                30005,
		NAT:                          true,
		Mining:                       true,
		MiningDebug:                  true,
		LogLevel:                     1,
		MaxLogFileSize:               20,
		MaxLogFileTotalSize:          100,
		SyncStateMaxThread:           0,
		SyncHeaderMaxSize:            0,
		SyncHeaderMaxMemorySize:      0,
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
		RegisterIDReplaceTxPool:      false,
		LogPath:                      "Log",
		ChainDBPath:                  "ChainDB",
		WalletFile:                   "wallet.json",
		DefaultTlsDomainTmpl:         "{{.DashedIP}}.ipv4.nknlabs.io",
		CertDirectory:                "certs",
		CertRenewBefore:              720,
		CertCheckInterval:            86400,
		MaxGetIDSeeds:                3,
		DBFilesCacheCapacity:         100,
		NumLowFeeTxnPerBlock:         0,
		LowFeeTxnSizePerBlock:        2048,
		LowTxnFee:                    10000000,
		LowTxnFeePerSize:             50000,
		AllowEmptyBeneficiaryAddress: false,
		WebGuiListenAddress:          "127.0.0.1",
		WebGuiPort:                   30000,
		WebGuiCreateWallet:           false,
		PasswordFile:                 "",
		StatePruningMode:             "lowmem",
		RecentStateCount:             16384,
		RecentBlockCount:             32768,
		MinPruningCompactHeights:     32768,
		NodeIPRateLimit:              1,
		NodeIPRateBurst:              10,
		WsIPRateLimit:                10,
		WsIPRateBurst:                100,
		RPCIPRateLimit:               10,
		RPCIPRateBurst:               100,
		RPCRateLimit:                 1024,
		RPCRateBurst:                 4096,
		SyncBlockHeaderRateLimit:     8192,
		SyncBlockHeaderRateBurst:     32768,
		SyncBlockRateLimit:           256,
		SyncBlockRateBurst:           1024,
		SyncMode:                     "full",
		MaxRollbackBlocks:            180,
	}
)

type Configuration struct {
	Version                      int           `json:"Version"`
	SeedList                     []string      `json:"SeedList"`
	HttpWssDomain                string        `json:"HttpWssDomain"`
	HttpWssCert                  string        `json:"HttpWssCert"`
	HttpWssKey                   string        `json:"HttpWssKey"`
	HttpWsPort                   uint16        `json:"HttpWsPort"`
	HttpWssPort                  uint16        `json:"HttpWssPort"`
	HttpJsonPort                 uint16        `json:"HttpJsonPort"`
	HttpsJsonDomain              string        `json:"HttpsJsonDomain"`
	HttpsJsonCert                string        `json:"HttpsJsonCert"`
	HttpsJsonKey                 string        `json:"HttpsJsonKey"`
	HttpsJsonPort                uint16        `json:"HttpsJsonPort"`
	DefaultTlsCert               string        `json:"DefaultTlsCert"`
	DefaultTlsKey                string        `json:"DefaultTlsKey"`
	DefaultTlsDomainTmpl         string        `json:"DefaultTlsDomainTmpl"`
	ACMEUserFile                 string        `json:"ACMEUserFile"`
	ACMEResourceFile             string        `json:"ACMEResourceFile"`
	CertRenewBefore              uint16        `json:"CertRenewBefore"`   //in hours
	CertCheckInterval            time.Duration `json:"CertCheckInterval"` // in seconds
	CertDirectory                string        `json:"CertDirectory"`
	NodePort                     uint16        `json:"-"`
	LogLevel                     int           `json:"LogLevel"`
	MaxLogFileSize               uint32        `json:"MaxLogSize"`
	MaxLogFileTotalSize          uint32        `json:"MaxLogFileTotalSize"`
	GenesisBlockProposer         string        `json:"GenesisBlockProposer"`
	NumLowFeeTxnPerBlock         uint32        `json:"NumLowFeeTxnPerBlock"`
	LowFeeTxnSizePerBlock        uint32        `json:"LowFeeTxnSizePerBlock"` // in bytes
	LowTxnFee                    int64         `json:"LowTxnFee"`
	LowTxnFeePerSize             float64       `json:"LowTxnFeePerSize"`
	RegisterIDRegFee             int64         `json:"RegisterIDRegFee"`
	RegisterIDTxnFee             int64         `json:"RegisterIDTxnFee"`
	RegisterIDReplaceTxPool      bool          `json:"RegisterIDReplaceTxPool"`
	Hostname                     string        `json:"Hostname"`
	Transport                    string        `json:"Transport"`
	NAT                          bool          `json:"NAT"`
	Mining                       bool          `json:"Mining"`
	MiningDebug                  bool          `json:"MiningDebug"`
	BeneficiaryAddr              string        `json:"BeneficiaryAddr"`
	SyncStateMaxThread           uint32        `json:"SyncStateMaxThread"`
	SyncHeaderMaxSize            uint32        `json:"SyncHeaderMaxSize"`
	SyncHeaderMaxMemorySize      uint32        `json:"SyncHeaderMaxMemorySize"`
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
	StatePruningMode             string        `json:"StatePruningMode"`
	RecentStateCount             uint32        `json:"RecentStateCount"`
	RecentBlockCount             uint32        `json:"RecentBlockCount"`
	MinPruningCompactHeights     uint32        `json:"MinPruningCompactHeights"`
	NodeIPRateLimit              float64       `json:"NodeIPRateLimit"` // requests per second
	NodeIPRateBurst              uint32        `json:"NodeIPRateBurst"`
	WsIPRateLimit                float64       `json:"WsIPRateLimit"` // requests per second
	WsIPRateBurst                uint32        `json:"WsIPRateBurst"`
	RPCIPRateLimit               float64       `json:"RPCIPRateLimit"` // requests per second
	RPCIPRateBurst               uint32        `json:"RPCIPRateBurst"`
	RPCRateLimit                 float64       `json:"RPCRateLimit"` // requests per second
	RPCRateBurst                 uint32        `json:"RPCRateBurst"`
	SyncBlockHeaderRateLimit     float64       `json:"SyncBlockHeaderRateLimit"` // headers per second
	SyncBlockHeaderRateBurst     uint32        `json:"SyncBlockHeaderRateBurst"`
	SyncBlockRateLimit           float64       `json:"SyncBlockRateLimit"` // blocks per second
	SyncBlockRateBurst           uint32        `json:"SyncBlockRateBurst"`
	BlockHeaderCacheSize         uint32        `json:"BlockHeaderCacheSize"`
	SigChainCacheSize            uint32        `json:"SigChainCacheSize"`
	SyncMode                     string        `json:"SyncMode"`
	MaxRollbackBlocks            uint32        `json:"MaxRollbackBlocks"`
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

	if len(SyncMode) > 0 {
		Parameters.SyncMode = SyncMode
	}

	if Parameters.SyncStateMaxThread == 0 {
		syncStateMaxPeer := uint32(float64(memory.TotalMemory() / defaultSyncStateThreadMemory))
		Parameters.SyncStateMaxThread = syncStateMaxPeer
		if Parameters.SyncStateMaxThread == 0 {
			Parameters.SyncStateMaxThread = defaultSyncStateMaxThread
		}
		log.Printf("Set SyncStateMaxThread to %v", Parameters.SyncStateMaxThread)
	}

	if Parameters.SyncHeaderMaxSize == 0 {
		syncHeaderMaxMemorySize := uint64(Parameters.SyncHeaderMaxMemorySize) * 1024 * 1024
		if syncHeaderMaxMemorySize == 0 {
			syncHeaderMaxMemorySize = uint64(float64(memory.TotalMemory()) * defaultSyncHeaderMaxMemoryPercent / 100.0)
		}
		Parameters.SyncHeaderMaxSize = uint32(syncHeaderMaxMemorySize / estimatedHeaderSize)
		if Parameters.SyncHeaderMaxSize == 0 {
			Parameters.SyncHeaderMaxSize = defaultSyncHeaderMaxSize
		}
		log.Printf("Set SyncHeaderMaxSize to %v", Parameters.SyncHeaderMaxSize)
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

	if Parameters.BlockHeaderCacheSize < uint32(SigChainRecentMinerBlocks+64) {
		Parameters.BlockHeaderCacheSize = uint32(SigChainRecentMinerBlocks + 64)
	}

	if Parameters.BlockHeaderCacheSize < uint32(SigChainSkipMinerBlocks+64) {
		Parameters.BlockHeaderCacheSize = uint32(SigChainSkipMinerBlocks + 64)
	}

	if Parameters.SigChainCacheSize < uint32(SigChainSkipMinerBlocks+64) {
		Parameters.SigChainCacheSize = uint32(SigChainSkipMinerBlocks + 64)
	}

	err = Parameters.verify()
	if err != nil {
		return err
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

	pk, err := hex.DecodeString(config.GenesisBlockProposer)
	if err != nil {
		return fmt.Errorf("parse GenesisBlockProposer error: %v", err)
	}
	if len(pk) != PublicKeySize {
		return fmt.Errorf("invalid GenesisBlockProposer length %d bytes, expecting %d bytes", len(pk), PublicKeySize)
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

	if config.MaxLogFileTotalSize <= 0 {
		return fmt.Errorf("MaxLogFileTotalSize should be >= 1 (MB)")
	}

	switch config.StatePruningMode {
	case "lowmem":
	case "none":
	default:
		return fmt.Errorf("unknown state pruning mode %v", config.StatePruningMode)
	}

	switch config.SyncMode {
	case "light":
	case "fast":
	case "full":
	default:
		return fmt.Errorf("unknown sync mode %v", config.SyncMode)
	}

	if config.RecentBlockCount <= GenerateIDBlockDelay {
		return fmt.Errorf("RecentBlockCount should be > %d", GenerateIDBlockDelay)
	}

	if config.RecentBlockCount <= config.SigChainCacheSize {
		return fmt.Errorf("RecentBlockCount should be > %d", config.SigChainCacheSize)
	}

	if config.RecentBlockCount <= config.MaxRollbackBlocks {
		return fmt.Errorf("RecentBlockCount should be > %d", config.MaxRollbackBlocks)
	}

	return nil
}

func (config *Configuration) IsFastSync() bool {
	return config.SyncMode == "fast" || config.SyncMode == "light"
}

func (config *Configuration) IsLightSync() bool {
	return config.SyncMode == "light"
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
		config.HttpWssPort,
		config.HttpsJsonPort,
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
	config.HttpWssPort += delta
	config.HttpJsonPort += delta
	config.HttpsJsonPort += delta
	if delta > 0 {
		log.Println("Port in use! All ports are automatically increased by", delta)
	}
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
			return errors.New("beneficiary address is empty")
		}
	}

	if addr != "" {
		_, err := common.ToScriptHash(addr)
		if err != nil {
			return fmt.Errorf("parse BeneficiaryAddr error: %v", err)
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

	configuration["BeneficiaryAddr"] = addr

	err = WriteConfigFile(configuration)
	if err != nil {
		return err
	}
	Parameters.BeneficiaryAddr = addr
	return nil
}
