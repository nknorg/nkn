package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/nknorg/nkn/api/common"
	"github.com/nknorg/nkn/api/httpjson"
	"github.com/nknorg/nkn/api/httpjson/client"
	"github.com/nknorg/nkn/api/websocket"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/chain/db"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/consensus"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/dashboard"
	serviceConfig "github.com/nknorg/nkn/dashboard/config"
	"github.com/nknorg/nkn/node"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/password"
	"github.com/nknorg/nkn/vault"
	"github.com/nknorg/nnet"
	nnetnode "github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay"
	"github.com/nknorg/nnet/overlay/chord"
	ipify "github.com/rdegges/go-ipify"
	"github.com/urfave/cli"
)

const (
	NetVersionNum = 3 // This is temporary and will be removed soon after mainnet is stabilized
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
	store, err := db.NewLedgerStore()
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
	por.Store = chain.DefaultLedger.Store

	return nil
}

func JoinNet(nn *nnet.NNet) error {
	seeds := config.Parameters.SeedList
	rand.Shuffle(len(seeds), func(i int, j int) {
		seeds[i], seeds[j] = seeds[j], seeds[i]
	})

	for _, seed := range seeds {
		succAddrs, err := client.FindSuccessorAddrs(seed, nn.GetLocalNode().Id)
		if err != nil {
			log.Warningf("Can't get successor address from [%s]", seed)
			continue
		}

		success := false
		for _, succAddr := range succAddrs {
			if succAddr == nn.GetLocalNode().Addr {
				log.Warning("Skipping self...")
				continue
			}
			err = nn.Join(succAddr)
			if err != nil {
				log.Error(err)
				continue
			}
			success = true
		}

		if success {
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
		log.Warningf("Ask my ID from %s met error: %v", seed, err)
	}
	return "", errors.New("Tried all seeds but can't got my external IP and nknID")
}

func nknMain(c *cli.Context) error {
	signalChan := make(chan os.Signal, 1)

	err := config.Init()
	if err != nil {
		return err
	}
	defer config.Parameters.CleanPortMapping()

	err = log.Init()
	if err != nil {
		return err
	}

	// start webservice
	go dashboard.Start()

	log.Infof("Node version: %v", config.Version)

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

	var wallet vault.Wallet
	var account *vault.Account
	for wallet == nil {
		time.Sleep(time.Second * 3)
		wallet, err = vault.GetWallet()
		if err != nil {
			fmt.Println(err)
			continue
		}
		account, err = wallet.GetDefaultAccount()
		if err != nil {
			fmt.Println(err)
		}
	}

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

	// initialize ledger
	err = InitLedger(account)
	if err != nil {
		return fmt.Errorf("chain.initialization error: %v", err)
	}
	// if InitLedger return err, chain.DefaultLedger is uninitialized.
	defer chain.DefaultLedger.Store.Close()

	id, err := GetOrCreateID(config.Parameters.SeedList, wallet, Fixed64(config.Parameters.RegisterIDFee))
	if err != nil {
		panic(err.Error())
	}

	log.Info("current chord ID: ", BytesToHexString(id))

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

	err = por.InitPorServer(account, id)
	if err != nil {
		return errors.New("PorServer initialization error")
	}

	localNode, err := node.NewLocalNode(wallet, nn)
	if err != nil {
		return err
	}

	err = localNode.Start()
	if err != nil {
		return err
	}

	//init web service
	dashboard.Init(localNode, wallet)

	//start JsonRPC
	rpcServer := httpjson.NewServer(localNode, wallet)

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
			for {
				time.Sleep(time.Minute)
				err = fmt.Errorf("Node has no neighbors and is too lonely to run")
				if localNode.GetConnectionCnt() == 0 {
					panic(err)
				}
				if nnetNeighbors, _ := nn.GetLocalNode().GetNeighbors(nil); len(nnetNeighbors) == 0 {
					panic(err)
				}
			}
		}()
	}

	defer nn.Stop(nil)

	go rpcServer.Start()

	go ws.Start()

	consensus, err := moca.NewConsensus(account, localNode)
	if err != nil {
		return err
	}

	consensus.Start()

	signal.Notify(signalChan, os.Interrupt)
	for _ = range signalChan {
		fmt.Println("\nReceived an interrupt, stopping services...\n")
		return nil
	}

	return nil
}

type NetVer struct {
	Ver int `json:"version"`
}

func GetRemoteVersionNum() (int, error) {
	var myClient = &http.Client{Timeout: 10 * time.Second}
	r, err := myClient.Get("https://nkn.org/mainnet.runtime.version")
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
				log.Error("Your current nknd is deprecated, Please download the latest NKN software from https://github.com/nknorg/nkn/releases")
				panic("Your current nknd is deprecated, Please download the latest NKN software from https://github.com/nknorg/nkn/releases")
			}

			timer.Reset(30 * time.Minute)
		}
	}
}

func main() {
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
			Name:        "remote",
			Usage:       "web service was run in remote mode",
			Destination: &serviceConfig.IsRemote,
		},
		cli.BoolFlag{
			Name:        "onlyui",
			Usage:       "only run web ui",
			Destination: &config.OnlyUI,
		},
	}
	app.Action = nknMain

	// app.Run will shutdown graceful.
	if err := app.Run(os.Args); err != nil {
		log.Errorf("%v", err)
		os.Exit(1)
	}
}

func GetID(seeds []string, publickey []byte) ([]byte, error) {
	id, err := chain.DefaultLedger.Store.GetID(publickey)
	if err == nil && len(id) != 0 {
		return id, nil
	}

	if err != nil {
		log.Errorf("get ID from local ledger error: %v", err)
	} else {
		log.Infof("get no ID from local ledger")
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

func CreateID(seeds []string, wallet vault.Wallet, regFee Fixed64) error {
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

	for _, seed := range seeds {
		nonce, err := client.GetNonceByAddr(seed, addr)
		if err != nil {
			log.Warningf("get nonce from %s met error: %v", seed, err)
			continue
		}

		txn, err := common.MakeGenerateIDTransaction(wallet, regFee, nonce, 0)
		if err != nil {
			return err
		}

		buff, err := txn.Marshal()
		if err != nil {
			return err
		}

		_, err = client.CreateID(seed, hex.EncodeToString(buff))
		if err != nil {
			log.Warningf("create ID from %s met error: %v", seed, err)
			continue
		}

		return nil
	}

	return errors.New("create ID failed")
}

func GetOrCreateID(seeds []string, wallet vault.Wallet, regFee Fixed64) ([]byte, error) {
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	pk := account.PubKey().EncodePoint()

	id, err := GetID(seeds, pk)
	if err != nil || id == nil {
		if err != nil {
			log.Warningf("Get id from neighbors error: %v", err)
		}
		if err := CreateID(seeds, wallet, regFee); err != nil {
			return nil, err
		}
	} else if len(id) != config.NodeIDBytes {
		return nil, fmt.Errorf("Got id %x from neighbors with wrong size, expecting %d bytes", id, config.NodeIDBytes)
	} else if !bytes.Equal(id, crypto.Sha256ZeroHash) {
		return id, nil
	}

	timer := time.NewTimer((config.GenerateIDBlockDelay + 2) * config.ConsensusDuration)
	timeout := time.After((config.GenerateIDBlockDelay + 5) * config.ConsensusTimeout)
	defer timer.Stop()

out:
	for {
		select {
		case <-timer.C:
			timer.Reset(config.ConsensusDuration)
			log.Warningf("try to get ID from local ledger and remoteNode...")
			if id, err := GetID(seeds, pk); err == nil && id != nil {
				if !bytes.Equal(id, crypto.Sha256ZeroHash) {
					return id, nil
				}
			} else if err != nil {
				log.Warningf("Get id from neighbors error: %v", err)
			}
		case <-timeout:
			break out
		}
	}

	return nil, errors.New("get ID timeout")
}
