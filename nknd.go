package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/nknorg/nkn/api/httpjson"
	"github.com/nknorg/nkn/api/httpjson/client"
	"github.com/nknorg/nkn/api/websocket"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/chain/db"
	"github.com/nknorg/nkn/consensus"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/node"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/address"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/password"
	"github.com/nknorg/nkn/vault"
	"github.com/nknorg/nnet"
	nnetnode "github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay"
	"github.com/nknorg/nnet/overlay/chord"
	"github.com/urfave/cli"
)

const (
	TestNetVersionNum = 12
)

var (
	createMode bool
	seedStr    string
)

func init() {
	log.Init(log.Path, log.Stdout)
	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UnixNano())
	crypto.SetAlg(config.EncryptAlg)
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
	return errors.New("Failed to join the network.")
}

// AskMyID request to seeds randomly, in order to obtain self's externIP and corresponding chordID
func AskMyID(seeds []string) (string, error) {
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
	log.Info("Node version: ", config.Version)
	signalChan := make(chan os.Signal, 1)

	err := config.Init()
	if err != nil {
		return err
	}
	log.Log.SetDebugLevel(config.Parameters.LogLevel) // Update LogLevel after config.json loaded

	defer config.Parameters.CleanPortMapping()

	if config.Parameters.Hostname == "" {
		log.Info("Getting my IP address...")
		extIP, err := AskMyID(config.Parameters.SeedList)
		if err != nil {
			return err
		}
		config.Parameters.Hostname = extIP
	}

	// Get local account
	wallet := vault.GetWallet()
	if wallet == nil {
		return errors.New("open local wallet error")
	}
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return errors.New("load local account error")
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

	id := address.GenChordID(fmt.Sprintf("%s:%d", config.Parameters.Hostname, config.Parameters.NodePort))
	nn, err := nnet.NewNNet(id, conf)
	if err != nil {
		return err
	}

	nn.MustApplyMiddleware(overlay.NetworkStopped(func(network overlay.Network) bool {
		select {
		case signalChan <- os.Interrupt:
		default:
		}
		return true
	}))

	if len(seedStr) > 0 {
		// Support input mutil seeds which split by ","
		config.Parameters.SeedList = strings.Split(seedStr, ",")
	}

	// initialize ledger
	err = InitLedger(account)
	if err != nil {
		return fmt.Errorf("chain.initialization error: %v", err)
	}
	// if InitLedger return err, chain.DefaultLedger is uninitialized.
	defer chain.DefaultLedger.Store.Close()

	err = por.InitPorServer(account, id)
	if err != nil {
		return fmt.Errorf("PorServer initialization error with: %v", err)
	}

	localNode, err := node.NewLocalNode(wallet, nn)
	if err != nil {
		return err
	}

	err = localNode.Start()
	if err != nil {
		return err
	}

	//start JsonRPC
	rpcServer := httpjson.NewServer(localNode, wallet)

	// start websocket server
	ws := websocket.NewServer(localNode, wallet)

	nn.MustApplyMiddleware(chord.SuccessorAdded(func(remoteNode *nnetnode.RemoteNode, index int) bool {
		if index == 0 {
			ws.NotifyWrongClients()
		}
		return true
	}))

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

	go func() {
		for {
			time.Sleep(config.ConsensusDuration)
			if log.CheckIfNeedNewFile() {
				log.ClosePrintLog()
				log.Init(log.Path, os.Stdout)
			}
		}
	}()

	signal.Notify(signalChan, os.Interrupt)
	for _ = range signalChan {
		fmt.Println("\nReceived an interrupt, stopping services...\n")
		return nil
	}

	return nil
}

type testNetVer struct {
	Ver int `json:"version"`
}

func GetRemoteVersionNum() (int, error) {
	var myClient = &http.Client{Timeout: 10 * time.Second}
	r, err := myClient.Get("http://testnet.nkn.org/nkn.runtime.version")
	if err != nil {
		return 0, err
	}
	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return 0, err
	}
	res := testNetVer{}
	json.Unmarshal(body, &res)

	return res.Ver, err
}

func TestNetVersion(timer *time.Timer) {
	for {
		select {
		case <-timer.C:
			verNum, err := GetRemoteVersionNum()
			if err != nil {
				log.Warning("Get the remote version number error")
				timer.Reset(30 * time.Minute)
				break
			}
			if verNum > TestNetVersionNum {
				log.Error("Your current nknd is deprecated, Please download the latest NKN software from https://github.com/nknorg/nkn/releases")
				panic("Your current nknd is deprecated, Please download the latest NKN software from https://github.com/nknorg/nkn/releases")
			}

			timer.Reset(30 * time.Minute)
		}
	}
}

func main() {
	// Detect the remote nknd version, only used for testnet for debugging purposes
	timer := time.NewTimer(1 * time.Second)
	go TestNetVersion(timer)

	app := cli.NewApp()
	app.Name = "nknd"
	app.Version = config.Version
	app.HelpName = "nknd"
	app.Usage = "full node of NKN blockchain"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "create, c",
			Usage:       "Create Mode",
			Hidden:      false,
			Destination: &createMode,
		},
		cli.StringFlag{
			Name:        "seed",
			Usage:       "Seed List to join",
			Value:       "",
			Destination: &seedStr,
		},
		cli.StringFlag{
			Name:        "passwd, p",
			Usage:       "Password of Your wallet private Key",
			Value:       "",
			Hidden:      true,
			Destination: &password.Passwd,
		},
		cli.BoolFlag{
			Name:        "no-check-port",
			Usage:       "Skip checking port opening",
			Hidden:      true,
			Destination: &config.SkipCheckPort,
		},
		cli.BoolFlag{
			Name:        "no-nat",
			Usage:       "Skip NAT traversal for UPnP and NAT-PMP",
			Hidden:      false,
			Destination: &config.SkipNAT,
		},
	}
	app.Action = nknMain

	// app.Run will shutdown graceful.
	if err := app.Run(os.Args); err != nil {
		log.Errorf("%v", err)
		os.Exit(1)
	}
}
