package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/rdegges/go-ipify"

	"github.com/nknorg/nkn/v2/api/certs"
	api "github.com/nknorg/nkn/v2/api/common"
	"github.com/nknorg/nkn/v2/api/httpjson"
	"github.com/nknorg/nkn/v2/api/httpjson/client"
	"github.com/nknorg/nkn/v2/api/websocket"
	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/chain/store"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/consensus"
	"github.com/nknorg/nkn/v2/crypto"
	"github.com/nknorg/nkn/v2/dashboard"
	serviceConfig "github.com/nknorg/nkn/v2/dashboard/config"
	"github.com/nknorg/nkn/v2/node"
	"github.com/nknorg/nkn/v2/por"
	"github.com/nknorg/nkn/v2/transaction"
	"github.com/nknorg/nkn/v2/util"
	"github.com/nknorg/nkn/v2/util/log"
	"github.com/nknorg/nkn/v2/util/password"
	"github.com/nknorg/nkn/v2/vault"
	"github.com/nknorg/nnet"
	nnetnode "github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay"
	"github.com/nknorg/nnet/overlay/chord"
	"github.com/urfave/cli"
)

const (
	NetVersionNum = 28 // This is temporary and will be removed soon after mainnet is stabilized
)

var (
	createMode bool
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UnixNano())
}

func InitLedger(account *vault.Account) error {
	var err error
	store, err := store.NewLedgerStore()
	if err != nil {
		return err
	}
	blockChain, err := chain.NewBlockchainWithGenesisBlock(store)
	if err != nil {
		return err
	}
	chain.DefaultLedger = &chain.Ledger{
		Blockchain: blockChain,
		Store:      store,
	}

	return nil
}

func printMemStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Infof("Alloc = %v TotalAlloc = %v Sys = %v NumGC = %v\n", m.Alloc/1024, m.TotalAlloc/1024, m.Sys/1024, m.NumGC)
	log.Infof("HeapAlloc = %v HeapSys = %v HeapIdle = %v HeapInuse = %v HeapReleased = %v HeapObjects = %v\n", m.HeapAlloc/1024, m.HeapSys/1024, m.HeapIdle/1024, m.HeapInuse/1024, m.HeapReleased/1024, m.HeapObjects/1024)
	log.Infof("StackInuse = %v StackSys = %v MCacheInuse = %v MCacheSys = %v\n", m.StackInuse/1024, m.StackSys/1024, m.MCacheInuse/1024, m.MCacheSys/1024)
}

func JoinNet(nn *nnet.NNet) error {
	seeds := config.Parameters.SeedList
	rand.Shuffle(len(seeds), func(i int, j int) {
		seeds[i], seeds[j] = seeds[j], seeds[i]
	})

	for _, seed := range seeds {
		randAddrs, err := client.FindSuccessorAddrs(seed, util.RandomBytes(config.NodeIDBytes))
		if err != nil {
			log.Warningf("Can't get successor address from [%s]", seed)
			continue
		}

		for _, randAddr := range randAddrs {
			if randAddr == nn.GetLocalNode().Addr {
				log.Warning("Skipping self...")
				continue
			}
			err = nn.Join(randAddr)
			if err != nil {
				log.Error(err)
				continue
			}
			return nil
		}
	}
	return errors.New("Failed to join the network")
}

// AskMyIP request to seeds randomly, in order to obtain self's externIP and corresponding chordID
func AskMyIP(seeds []string) (string, error) {
	rand.Shuffle(len(seeds), func(i int, j int) {
		seeds[i], seeds[j] = seeds[j], seeds[i]
	})

	for _, seed := range seeds {
		addr, err := client.GetMyExtIP(seed, []byte{})
		if err == nil {
			return addr, err
		}
		log.Warningf("Ask my ID from %s error: %v", seed, err)
	}
	return "", errors.New("Tried all seeds but can't got my external IP and nknID")
}

func nknMain(c *cli.Context) error {
	if config.Debug {
		//pprof
		go func() {
			log.Info(http.ListenAndServe(config.PprofPort, nil))
		}()

		//dump runtime memory status
		t := time.NewTicker(config.DumpMemInterval)
		go func() {
			for {
				<-t.C
				printMemStats()
			}
		}()
	}

	signalChan := make(chan os.Signal, 1)

	err := config.Init()
	if err != nil {
		return err
	}

	if err := SetupPortMapping(); err != nil {
		log.Errorf("Error setting up port mapping: %v. If this problem persists, you can use --no-nat flag to bypass automatic port forwarding and set it up yourself.", err)
	}

	defer func() {
		err := ClearPortMapping()
		if err != nil {
			log.Errorf("Error clear port mapping: %v", err)
		}
	}()

	err = log.Init()
	if err != nil {
		return err
	}

	// start webservice
	go dashboard.Start()

	log.Infof("Node version: %v", config.Version)

	var wallet *vault.Wallet
	var account *vault.Account
	if config.Parameters.WebGuiCreateWallet {
		for wallet == nil || account == nil {
			time.Sleep(time.Second * 3)

			wallet, err = vault.GetWallet(password.GetAccountPassword)
			if err != nil {
				fmt.Println(err)
				continue
			}

			account, err = wallet.GetDefaultAccount()
			if err != nil {
				fmt.Println(err)
			}
		}
	} else {
		// Get local account
		wallet, err = vault.GetWallet(password.GetAccountPassword)
		if err != nil {
			return err
		}
		account, err = wallet.GetDefaultAccount()
		if err != nil {
			return errors.New("load local account error")
		}
	}

	if config.Parameters.Hostname == "" { // Skip query self extIP via set "HostName" in config.json
		log.Info("Getting my IP address...")
		var extIP string
		if createMode { // There is no seed available in create mode, used ipify
			extIP, err = ipify.GetIp()
		} else {
			extIP, err = AskMyIP(config.Parameters.SeedList)
		}
		if err != nil {
			return err
		}
		log.Infof("My IP address is %s", extIP)
		config.Parameters.Hostname = extIP
	}

	wssCertReady, httpsCertReady := certs.PrepareCerts()

	// init web service
	dashboard.Init(nil, wallet, nil)

	// start JsonRPC
	rpcServer := httpjson.NewServer(nil, wallet)
	rpcServer.Start(httpsCertReady)

	// initialize ledger
	err = InitLedger(account)
	if err != nil {
		return fmt.Errorf("chain.initialization error: %v", err)
	}

	// if InitLedger return err, chain.DefaultLedger is uninitialized.
	defer chain.DefaultLedger.Store.Close()

	id, err := GetOrCreateID(
		config.Parameters.SeedList,
		wallet,
		common.Fixed64(config.Parameters.RegisterIDTxnFee),
		createMode,
	)
	if err != nil {
		log.Fatalf("Get or create id error: %v", err)
	}

	log.Infof("current chord ID: %x", id)

	conf := &nnet.Config{
		Transport:        config.Parameters.Transport,
		Hostname:         config.Parameters.Hostname,
		Port:             config.Parameters.NodePort,
		NodeIDBytes:      config.NodeIDBytes,
		MinNumSuccessors: config.MinNumSuccessors,
	}

	err = nnet.SetLogger(log.Log)
	if err != nil {
		return err
	}

	nn, err := nnet.NewNNet(id, conf)
	if err != nil {
		return err
	}

	nn.MustApplyMiddleware(overlay.NetworkStopped{func(network overlay.Network) bool {
		select {
		case signalChan <- os.Interrupt:
		default:
		}
		return true
	}, 0})

	localNode, err := node.NewLocalNode(wallet, nn)
	if err != nil {
		return err
	}

	err = por.InitPorServer(account, id, chain.DefaultLedger.Store, localNode)
	if err != nil {
		return errors.New("porServer initialization error")
	}

	err = localNode.Start()
	if err != nil {
		return err
	}

	// set JsonRPC server localnode
	rpcServer.SetLocalNode(localNode)

	// set web service localnode
	dashboard.Init(localNode, nil, id)

	// start websocket server
	ws := websocket.NewServer(localNode, wallet)

	nn.MustApplyMiddleware(chord.SuccessorAdded{func(remoteNode *nnetnode.RemoteNode, index int) bool {
		if index == 0 {
			ws.NotifyWrongClients()
		}
		return true
	}, 0})

	err = nn.Start(createMode)
	if err != nil {
		return err
	}

	if !createMode {
		err = JoinNet(nn)
		if err != nil {
			return err
		}

		go func() {
			errMsg := "Node has lost connections to all neighbors. This is typically caused by loss of Internet or firewall."
			for {
				time.Sleep(time.Minute)
				if localNode.GetConnectionCnt() == 0 {
					log.Fatal(errMsg)
				}
				if nnetNeighbors, err := nn.GetLocalNode().GetNeighbors(nil); err == nil && len(nnetNeighbors) == 0 {
					log.Fatal(errMsg)
				}
			}
		}()
	}

	defer nn.Stop(nil)

	ws.Start(wssCertReady)

	consensus, err := consensus.NewConsensus(account, localNode)
	if err != nil {
		return err
	}

	consensus.Start()

	signal.Notify(signalChan, os.Interrupt)
	for range signalChan {
		fmt.Printf("\nReceived an interrupt, stopping services...\n")
		return nil
	}

	return nil
}

type NetVer struct {
	Ver int `json:"version"`
}

func GetRemoteVersionNum() (int, error) {
	var myClient = &http.Client{Timeout: 10 * time.Second}
	r, err := myClient.Get("https://mainnet.nkn.org/version.json")
	if err != nil {
		return 0, err
	}
	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return 0, err
	}
	res := NetVer{}
	json.Unmarshal(body, &res)

	return res.Ver, err
}

// This is temporary and will be removed soon after mainnet is stabilized
func netVersion(timer *time.Timer) {
	for {
		select {
		case <-timer.C:
			verNum, err := GetRemoteVersionNum()
			if err != nil {
				log.Warningf("Get the remote version number error: %v", err)
				timer.Reset(30 * time.Minute)
				break
			}
			if verNum > NetVersionNum {
				log.Fatal("Your current nknd is deprecated, Please download the latest NKN software from https://github.com/nknorg/nkn/releases")
			}

			timer.Reset(30 * time.Minute)
		}
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Fatalf("Panic: %+v", r)
		}
	}()

	// This is temporary and will be removed soon after mainnet is stabilized
	timer := time.NewTimer(1 * time.Second)
	go netVersion(timer)

	app := cli.NewApp()
	app.Name = "nknd"
	app.Version = config.Version
	app.HelpName = "nknd"
	app.Usage = "full node of NKN blockchain"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "create, c",
			Usage:       "Create Mode",
			Destination: &createMode,
		},
		cli.StringFlag{
			Name:        "seed",
			Usage:       "Seed node address to join, multiple seeds should be split by comma",
			Destination: &config.SeedList,
		},
		cli.StringFlag{
			Name:        "passwd, p",
			Usage:       "Password of Your wallet private Key",
			Hidden:      true,
			Destination: &password.Passwd,
		},
		cli.BoolFlag{
			Name:        "no-nat",
			Usage:       "Skip NAT traversal for UPnP and NAT-PMP",
			Destination: &config.SkipNAT,
		},
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "Provide runtime profiling data of NKN",
			Destination: &config.Debug,
		},
		cli.StringFlag{
			Name:        "pruning",
			Usage:       "state pruning mode: none, lowmem",
			Destination: &config.StatePruningMode,
		},
		cli.StringFlag{
			Name:        "sync",
			Usage:       "sync mode: full, fast, light",
			Destination: &config.SyncMode,
		},
		cli.StringFlag{
			Name:        "pprof-port",
			Usage:       "The port used for pprof in debug mode",
			Destination: &config.PprofPort,
		},
		cli.StringFlag{
			Name:        "config",
			Usage:       "config file name",
			Destination: &config.ConfigFile,
		},
		cli.StringFlag{
			Name:        "log",
			Usage:       "directory where your log file will be generated",
			Destination: &config.LogPath,
		},
		cli.StringFlag{
			Name:        "chaindb",
			Usage:       "directory where your blockchain data will be stored",
			Destination: &config.ChainDBPath,
		},
		cli.StringFlag{
			Name:        "wallet",
			Usage:       "wallet file",
			Destination: &config.WalletFile,
		},
		cli.StringFlag{
			Name:        "beneficiaryaddr",
			Usage:       "beneficiary address where your mining reward will go to",
			Destination: &config.BeneficiaryAddr,
		},
		cli.StringFlag{
			Name:        "genesisblockproposer",
			Usage:       "public key of genesis block proposer",
			Destination: &config.GenesisBlockProposer,
		},
		cli.BoolFlag{
			Name:        "allow-empty-beneficiary-address",
			Usage:       "beneficiary address is forced unless --allow-empty-beneficiary-address is true",
			Destination: &config.AllowEmptyBeneficiaryAddress,
		},
		cli.StringFlag{
			Name:        "web-gui-listen-address",
			Usage:       "web gui will listen this address (default: 127.0.0.1)",
			Destination: &config.WebGuiListenAddress,
		},
		cli.BoolFlag{
			Name:        "web-gui-create-wallet",
			Usage:       "web gui create/open wallet",
			Destination: &config.WebGuiCreateWallet,
		},
		cli.StringFlag{
			Name:        "password-file",
			Usage:       "read password from file, save password to file when --web-gui-create-wallet arguments be true and password file does not exist",
			Destination: &config.PasswordFile,
		},
	}
	app.Action = nknMain

	// app.Run will shutdown graceful.
	if err := app.Run(os.Args); err != nil {
		log.Errorf("%v", err)
		os.Exit(1)
	}
}

func GetID(seeds []string, publickey []byte, createMode bool) ([]byte, error) {
	// Get future ID assuming ID will not expire
	height := uint32(math.MaxUint32)
	if createMode {
		height = chain.DefaultLedger.Store.GetHeight()
	}

	id, err := chain.DefaultLedger.Store.GetID(publickey, height)
	if err == nil && len(id) > 0 && !bytes.Equal(id, crypto.Sha256ZeroHash) {
		return id, nil
	}

	if err != nil {
		log.Errorf("get ID from local ledger error: %v", err)
	} else {
		log.Infof("get no ID from local ledger")
	}
	if createMode {
		return nil, errors.New("no ID in local ledger")
	}

	rand.Shuffle(len(seeds), func(i int, j int) {
		seeds[i], seeds[j] = seeds[j], seeds[i]
	})

	n := uint32(len(seeds))
	if n > config.Parameters.MaxGetIDSeeds {
		n = config.Parameters.MaxGetIDSeeds
	}

	counter := make(map[string]uint32)
	for i := uint32(0); i < n; i++ {
		id, err := client.GetID(seeds[i], publickey)
		if err == nil && id != nil {
			counter[string(id)]++
		} else {
			counter[""]++
		}
	}

	for idStr, count := range counter {
		if count > n/2 {
			if idStr == "" {
				return nil, nil
			}
			return []byte(idStr), nil
		}
	}

	return nil, fmt.Errorf("failed to get ID from majority of %d seeds", n)
}

func CreateID(seeds []string, wallet *vault.Wallet, txnFee common.Fixed64) error {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return err
	}

	addr, err := account.ProgramHash.ToAddress()
	if err != nil {
		return err
	}

	rand.Shuffle(len(seeds), func(i int, j int) {
		seeds[i], seeds[j] = seeds[j], seeds[i]
	})

	var prevNonce uint64
	var txn *transaction.Transaction

	for _, seed := range seeds {
		nonce, height, err := client.GetNonceByAddr(seed, addr)
		if err != nil {
			log.Warningf("get nonce from %s error: %v", seed, err)
			continue
		}

		if txn == nil || nonce != prevNonce {
			txn, err = api.MakeGenerateIDTransaction(context.Background(), nil, wallet, 0, nonce, txnFee, height)
			if err != nil {
				return err
			}
			prevNonce = nonce
		}

		buff, err := txn.Marshal()
		if err != nil {
			return err
		}

		_, err = client.CreateID(seed, hex.EncodeToString(buff))
		if err != nil {
			log.Warningf("create ID from %s error: %v", seed, err)
			continue
		}

		return nil
	}

	return errors.New("create ID failed")
}

func GetOrCreateID(seeds []string, wallet *vault.Wallet, txnFee common.Fixed64, createMode bool) ([]byte, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	pk := account.PubKey()

	for {
		id, err := GetID(seeds, pk, createMode)
		if err != nil || len(id) == 0 {
			if createMode {
				return nil, err
			}
			if err != nil {
				log.Warningf("Get ID from neighbors error: %v", err)
			}
			serviceConfig.Status = serviceConfig.Status | serviceConfig.SERVICE_STATUS_CREATE_ID
			if err := CreateID(seeds, wallet, txnFee); err != nil {
				log.Warningf("Create ID error: %v", err)
				log.Warningf("Failed to create ID. Make sure node's wallet address has enough balance for generate ID fee, or use another wallet to generate ID for this node's public key.")
				time.Sleep(10 * time.Minute)
				continue
			}
			break
		} else if len(id) != config.NodeIDBytes {
			return nil, fmt.Errorf("Got ID %x from neighbors with wrong size, expecting %d bytes", id, config.NodeIDBytes)
		} else if bytes.Equal(id, crypto.Sha256ZeroHash) {
			log.Info("Waiting for ID generation to complete")
			break
		}
		return id, nil
	}

	timer := time.NewTimer((config.GenerateIDBlockDelay + 4) * config.ConsensusDuration)
	timeout := time.After((config.GenerateIDBlockDelay + 12) * config.ConsensusTimeout)
	defer timer.Stop()

out:
	for {
		select {
		case <-timer.C:
			log.Infof("Try to get ID from local ledger and remoteNode...")
			if id, err := GetID(seeds, pk, false); err == nil && id != nil {
				if !bytes.Equal(id, crypto.Sha256ZeroHash) {
					return id, nil
				}
			} else if err != nil {
				log.Warningf("Get ID from neighbors error: %v", err)
			}
			timer.Reset(config.ConsensusDuration)
		case <-timeout:
			break out
		}
	}

	serviceConfig.Status = serviceConfig.Status &^ serviceConfig.SERVICE_STATUS_CREATE_ID
	return nil, errors.New("get ID timeout")
}
