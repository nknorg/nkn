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
	"github.com/nknorg/nkn/consensus/ising"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/db"
	"github.com/nknorg/nkn/net/node"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/password"
	"github.com/nknorg/nkn/vault"
	"github.com/nknorg/nnet"
	nnetconfig "github.com/nknorg/nnet/config"
	nnetnode "github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay/chord"
	"github.com/urfave/cli"
)

const (
	TestNetVersionNum = 1
)

var (
	createMode bool
	seedStr    string
)

func init() {
	log.Init(log.Path, log.Stdout)
	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UnixNano())
	crypto.SetAlg(config.Parameters.EncryptAlg)
}

func InitLedger(account *vault.Account) error {
	var err error
	store, err := db.NewLedgerStore()
	if err != nil {
		return err
	}
	ledger.StandbyBookKeepers = vault.GetBookKeepers(account)
	blockChain, err := ledger.NewBlockchainWithGenesisBlock(store, ledger.StandbyBookKeepers)
	if err != nil {
		return err
	}
	ledger.DefaultLedger = &ledger.Ledger{
		Blockchain: blockChain,
		Store:      store,
	}
	transaction.Store = ledger.DefaultLedger.Store
	por.Store = ledger.DefaultLedger.Store
	vault.Store = ledger.DefaultLedger.Store

	return nil
}

func StartConsensus(wallet vault.Wallet, node protocol.Noder) {
	log.Info("ising consensus starting ...")
	account, _ := wallet.GetDefaultAccount()
	go ising.NewProposerService(account, node).Start()
}

func JoinNet(nn *nnet.NNet) error {
	for _, seed := range config.Parameters.SeedList {
		info, err := client.GetNodeState(seed)
		if err != nil {
			log.Warningf("Can't get remote node info from [%s]", seed)
			continue
		}

		err = nn.Join(fmt.Sprintf("%s:%d", info.Addr, info.NodePort))
		if err == nil {
			return nil
		}
		log.Error(err)
	}
	return errors.New("Failed to join the network.")
}

func nknMain(c *cli.Context) error {
	log.Info("Node version: ", config.Version)

	err := config.Init()
	if err != nil {
		log.Error(err)
		os.Exit(1)
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

	conf := &nnetconfig.Config{
		Transport:        "tcp",
		Hostname:         config.Parameters.Hostname,
		Port:             config.Parameters.NodePort,
		NodeIDBytes:      32,
		MinNumSuccessors: 16,
		Logger:           log.Log,
	}

	id := node.GenChordID(fmt.Sprintf("%s:%d", config.Parameters.Hostname, config.Parameters.NodePort))

	nn, err := nnet.NewNNet(id, conf)
	if err != nil {
		return err
	}

	if len(seedStr) > 0 {
		// Support input mutil seeds which split by ","
		config.Parameters.SeedList = strings.Split(seedStr, ",")
	}

	// initialize ledger
	err = InitLedger(account)
	if err != nil {
		return fmt.Errorf("ledger initialization error: %v", err)
	}
	// if InitLedger return err, ledger.DefaultLedger is uninitialized.
	defer ledger.DefaultLedger.Store.Close()

	err = por.InitPorServer(account, id)
	if err != nil {
		return errors.New("PorServer initialization error")
	}

	node, err := node.InitNode(account.PublicKey, nn)
	if err != nil {
		return err
	}

	// start relay service
	node.StartRelayer(wallet)

	//start JsonRPC
	rpcServer := httpjson.NewServer(node, wallet)

	// start websocket server
	ws := websocket.NewServer(node, wallet)

	err = nn.ApplyMiddleware(chord.SuccessorAdded(func(remoteNode *nnetnode.RemoteNode, index int) bool {
		if index == 0 {
			ws.NotifyWrongClients()
		}
		return true
	}))
	if err != nil {
		return err
	}

	err = nn.Start()
	if err != nil {
		return err
	}

	if !createMode {
		err = JoinNet(nn)
		if err != nil {
			return err
		}
	}

	defer nn.Stop(nil)

	go rpcServer.Start()

	go ws.Start()

	// start consensus
	StartConsensus(wallet, node)

	go func() {
		for {
			time.Sleep(config.ConsensusTime)
			if log.CheckIfNeedNewFile() {
				log.ClosePrintLog()
				log.Init(log.Path, os.Stdout)
			}
		}
	}()

	signalChan := make(chan os.Signal, 1)
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
				log.Error("Your current nknd is deprecated")
				log.Error("Please download the latest NKN software from",
					"https://github.com/nknorg/nkn/releases")
				os.Exit(1)
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
	}
	app.Action = nknMain

	// app.Run will shutdown graceful.
	if err := app.Run(os.Args); err != nil {
		log.Errorf("%v", err)
		os.Exit(1)
	}
}
