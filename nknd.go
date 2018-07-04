package main

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/nknorg/nkn/api/httpjson"
	"github.com/nknorg/nkn/api/websocket"
	"github.com/nknorg/nkn/consensus/dbft"
	"github.com/nknorg/nkn/consensus/ising"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/db"
	"github.com/nknorg/nkn/net"
	"github.com/nknorg/nkn/net/chord"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/password"
	"github.com/nknorg/nkn/vault"
	"github.com/urfave/cli"
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
	transaction.TxStore = ledger.DefaultLedger.Store

	return nil
}

func StartNetworking(pubKey *crypto.PubKey, ring *chord.Ring) protocol.Noder {
	return net.StartProtocol(pubKey, ring)
}

func StartConsensus(wallet vault.Wallet, node protocol.Noder) {
	switch config.Parameters.ConsensusType {
	case "ising":
		log.Info("ising consensus starting ...")
		account, _ := wallet.GetDefaultAccount()
		go ising.StartIsingConsensus(account, node)
	case "dbft":
		log.Info("dbft consensus starting ...")
		dbftServices := dbft.NewDbftService(wallet, "logdbft", node)
		go dbftServices.Start()
	}
}

func nknMain(c *cli.Context) error {
	log.Info("Node version: ", config.Version)
	if len(seedStr) > 0 {
		// Support input mutil seeds which split by ","
		config.Parameters.SeedList = strings.Split(seedStr, ",")
	}

	var ring *chord.Ring
	var transport *chord.TCPTransport
	var err error

	// Start the Chord ring testing process
	if createMode {
		ring, transport, err = chord.CreateNet()
	} else {
		ring, transport, err = chord.JoinNet()
	}

	if err != nil {
		return err
	}

	defer transport.Shutdown()
	defer ring.Leave()

	// Get local account
	wallet := vault.GetWallet()
	if wallet == nil {
		return errors.New("open local wallet error")
	}
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return errors.New("load local account error")
	}

	// initialize ledger
	err = InitLedger(account)
	if err != nil {
		return errors.New("ledger initialization error")
	}
	// if InitLedger return err, ledger.DefaultLedger is uninitialized.
	defer ledger.DefaultLedger.Store.Close()

	err = por.InitPorServer(account)
	if err != nil {
		return errors.New("PorServer initialization error")
	}

	// start P2P networking
	node := StartNetworking(account.PublicKey, ring)

	// start relay service
	node.StartRelayer(wallet)

	//start JsonRPC
	rpcServer := httpjson.NewServer(node, wallet)
	go rpcServer.Start()

	// start websocket server
	ws := websocket.StartServer(node, wallet)

	vnode, err := ring.GetFirstVnode()
	if err != nil {
		return errors.New("Get first vnode in ring error")
	}
	vnode.OnNewSuccessor = func() {
		ws.CloseWrongClients()
	}

	// start consensus
	StartConsensus(wallet, node)

	go func() {
		for {
			time.Sleep(ising.ConsensusTime)
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

func main() {
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
	}
	app.Action = nknMain

	// app.Run will shutdown graceful.
	if err := app.Run(os.Args); err != nil {
		log.Errorf("%v", err)
		os.Exit(1)
	}
}
